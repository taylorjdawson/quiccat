package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"os"

	"github.com/lthibault/log"
	quic "github.com/lucas-clemente/quic-go"
	logutil "github.com/taylorjdawson/quiccat/internal/util/log"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
)

type quicClientConfig struct {
	fx.In
	C   *cli.Context
	Log log.Logger
}

type quicClient struct {
	log     log.Logger
	addr    string
	tls     *tls.Config
	quicCfg *quic.Config
	stream  quic.Stream

	abort context.CancelFunc
}

var flags = []cli.Flag{
	&cli.StringFlag{
		Name:    "address",
		Aliases: []string{"a"},
		Usage:   "address of the server",
		EnvVars: []string{"ADDRESS"},
	},
}

func Command() *cli.Command {
	return &cli.Command{
		Name:   "client",
		Usage:  "run a quic client",
		Flags:  flags,
		Action: run(),
	}
}

func run() cli.ActionFunc {
	return func(c *cli.Context) error {
		app := fx.New(fx.NopLogger,
			fx.Supply(c),
			fx.Provide(
				logutil.New,
				client,
			),
			fx.Invoke(entry))

		if err := app.Start(context.Background()); err != nil {
			return fmt.Errorf("start: %w", err)
		}

		<-app.Done()

		return app.Stop(context.Background())
	}
}

func entry(qc *quicClient, lx fx.Lifecycle) {
	var (
		cancel context.CancelFunc
	)

	lx.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ctx, cancel = context.WithCancel(ctx)
			return qc.Connect(ctx)
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			qc.Stop()
			return nil
		},
	})
}

func client(cfg quicClientConfig) *quicClient {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}
	return &quicClient{
		log:     cfg.Log.WithField("service", "client"),
		addr:    cfg.C.String("address"),
		tls:     tlsConf,
		quicCfg: &quic.Config{KeepAlive: true},
	}
}

func (qc *quicClient) Connect(ctx context.Context) error {
	ctx, qc.abort = context.WithCancel(ctx)
	defer qc.Stop()
	session, err := quic.DialAddr(qc.addr, qc.tls, nil)
	if err != nil {
		return err
	}

	qc.stream, err = session.OpenStreamSync(context.Background())
	if err != nil {
		return err
	}

	// Read from stdin and send messages
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			err = qc.Send([]byte(scanner.Bytes()))
			fmt.Print(">")
		}

		if scanner.Err() != nil {
			// Handle error.
		}
	}()

	go func() {

		//qc.log.WithField("received message:", buf).Debug()
		if _, err := io.Copy(os.Stdout, qc.stream); err != nil {
			fmt.Println(err)
			qc.log.WithError(err).Debug("error receiving message")
		}

	}()

	return nil
}

func (qc *quicClient) Send(message []byte) error {
	qc.log.WithField("sending message", message).Debug()
	_, err := qc.stream.Write([]byte(message))
	if err != nil {
		qc.log.WithError(err).WithField("message", message).Debug("error sending message")
		return err
	}
	return nil
}

func (qc *quicClient) Stop() {
	qc.log.Info("received abort signal")
	qc.abort()
}
