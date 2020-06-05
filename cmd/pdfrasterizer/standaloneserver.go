package main

import (
	"context"
	"log"
	"net/http"

	"github.com/function61/gokit/httputils"
	"github.com/function61/gokit/logex"
	"github.com/function61/gokit/osutil"
	"github.com/function61/gokit/taskrunner"
	"github.com/spf13/cobra"
)

func serverEntry() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Start server (also good for dev/testing)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			logger := logex.StandardLogger()

			osutil.ExitIfError(runServer(
				osutil.CancelOnInterruptOrTerminate(logger),
				logger))
		},
	}
}

func runServer(ctx context.Context, logger *log.Logger) error {
	srv := &http.Server{
		Addr:    ":80",
		Handler: newServerHandler(),
	}

	tasks := taskrunner.New(ctx, logger)

	tasks.Start("listener "+srv.Addr, func(_ context.Context) error {
		return httputils.RemoveGracefulServerClosedError(srv.ListenAndServe())
	})

	tasks.Start("listenershutdowner", httputils.ServerShutdownTask(srv))

	return tasks.Wait()
}
