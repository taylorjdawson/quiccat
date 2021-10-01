package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"

	"github.com/lthibault/log"
	quic "github.com/lucas-clemente/quic-go"
	logutil "github.com/taylorjdawson/quiccat/internal/util/log"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
)

var flags = []cli.Flag{
	&cli.StringFlag{
		Name:    "port",
		Aliases: []string{"p"},
		Usage:   "server listen port",
		EnvVars: []string{"PORT"},
	},
}

func Command() *cli.Command {
	return &cli.Command{
		Name:   "serve",
		Usage:  "run a quic server",
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
				server,
			),
			fx.Invoke(entry))

		if err := app.Start(context.Background()); err != nil {
			return fmt.Errorf("start: %w", err)
		}

		<-app.Done()

		return app.Stop(context.Background())
	}
}

func entry(qs *quicServer, lx fx.Lifecycle) {
	var (
		cancel context.CancelFunc
	)

	lx.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ctx, cancel = context.WithCancel(ctx)
			return qs.Serve(ctx)
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			qs.Stop()
			return nil
		},
	})
}

type quicServerConfig struct {
	fx.In

	Log log.Logger
	TLS *tls.Config
}

type quicServer struct {
	log     log.Logger
	addr    string
	tls     *tls.Config
	quicCfg *quic.Config

	abort context.CancelFunc
}

func server(cfg quicServerConfig) *quicServer {
	return &quicServer{
		log:     cfg.Log.WithField("service", "server"),
		tls:     generateTLSConfig(),
		quicCfg: &quic.Config{KeepAlive: true},
	}
}

func (qs *quicServer) Serve(ctx context.Context) error {
	ctx, qs.abort = context.WithCancel(ctx)
	defer qs.Stop()

	l, err := quic.ListenAddr(qs.addr, qs.tls, qs.quicCfg)
	if err != nil {
		return err
	}

	return handler{
		log: qs.log.WithField("listen", l.Addr()),
	}.HandleQUIC(ctx, l)
}

func (qs *quicServer) Stop() {
	qs.log.Info("received abort signal")
	qs.abort()
}

type handler struct {
	log log.Logger
}

func (h handler) HandleQUIC(ctx context.Context, l quic.Listener) error {
	h.log.Info("service started")
	defer h.log.Warn("service halted")

	for {
		sess, err := l.Accept(ctx)
		if err != nil {
			return err
		}

		go handler{
			log: h.log.WithField("remote", sess.RemoteAddr()),
		}.HandleSession(ctx, sess)
	}
}

func (h handler) HandleSession(ctx context.Context, sess quic.Session) {
	h.log.Debug("session established")
	defer h.log.Debug("session terminated")

	for {
		s, err := sess.AcceptStream(ctx)
		if err != nil {
			h.log.WithError(err).Debug("session error")
			return
		}
		h.log.WithField("stream-id", s.StreamID()).Info("stream accepted")
	}
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-echo-example"},
	}
}
