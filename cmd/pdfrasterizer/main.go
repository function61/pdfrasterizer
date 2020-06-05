package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/function61/gokit/aws/lambdautils"
	"github.com/function61/gokit/cryptorandombytes"
	"github.com/function61/gokit/dynversion"
	"github.com/function61/gokit/httputils"
	"github.com/function61/gokit/osutil"
	"github.com/function61/pdfrasterizer/pkg/pdfrasterizerclient"
	"github.com/spf13/cobra"
)

func main() {
	if lambdautils.InLambda() {
		lambda.StartHandler(lambdautils.NewLambdaHttpHandlerAdapter(newServerHandler()))
		return
	}

	app := &cobra.Command{
		Use:     os.Args[0],
		Short:   "PDF rasterizer",
		Version: dynversion.Version,
	}

	app.AddCommand(serverEntry())
	app.AddCommand(clientEntry("client-fn61", pdfrasterizerclient.Function61))
	app.AddCommand(clientEntry("client-localhost", pdfrasterizerclient.Localhost))

	osutil.ExitIfError(app.Execute())
}

func newServerHandler() http.Handler {
	mux := httputils.NewMethodMux()

	mux.POST.HandleFunc("/rasterize", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/pdf" {
			http.Error(w, "Required: 'Content-Type: application/pdf'", http.StatusBadRequest)
			return
		}

		// needs to be unique for each request. using named pipe because Ghostscript pollutes
		// stdout with log messages, and therefore it doesn't support writing to work output there
		outputPath, cleanup, err := randomFifoName()
		if err != nil {
			panic(err)
		}
		defer cleanup()

		pipingDone := make(chan error, 1)

		go func() {
			output, err := os.Open(outputPath)
			if err != nil {
				panic(err)
			}
			defer output.Close()

			_, err = io.Copy(w, output)
			pipingDone <- err
		}()

		ghostscript := exec.Command(
			"./gs",
			"-sDEVICE=jpeg",
			"-o", outputPath,
			"-dJPEGQ=95",
			"-dNOPAUSE",
			"-dBATCH",
			"-dUseCropBox",
			"-r140",
			"-", // = take from stdin
		)
		ghostscript.Stdin = r.Body
		defer r.Body.Close()

		w.Header().Set("Content-Type", "image/jpeg")

		if err := ghostscript.Run(); err != nil {
			panic(err)
		}

		if err := <-pipingDone; err != nil {
			log.Printf("pipe err: %v", err)
		}
	})

	return mux
}

func clientEntry(use string, baseUrl string) *cobra.Command {
	return &cobra.Command{
		Use:   use + " [path]",
		Short: "PDF rasterization via server",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			osutil.ExitIfError(client(
				osutil.CancelOnInterruptOrTerminate(nil),
				args[0],
				os.Stdout,
				baseUrl))
		},
	}
}

func client(ctx context.Context, path string, output io.Writer, baseUrl string) error {
	client, err := pdfrasterizerclient.New(baseUrl, pdfrasterizerclient.TokenFromEnv)
	if err != nil {
		return err
	}

	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer input.Close()

	rasterized, err := client.Rasterize(ctx, input)
	if err != nil {
		return err
	}
	defer rasterized.Close()

	_, err = io.Copy(output, rasterized)
	return err
}

func randomFifoName() (string, func(), error) {
	randomPath := filepath.Join("/tmp", cryptorandombytes.Base64Url(8))

	if err := syscall.Mkfifo(randomPath, 0600); err != nil {
		return "", nil, err
	}

	return randomPath, func() {
		if err := os.Remove(randomPath); err != nil {
			log.Printf("randomFifoName cleanup: %v", err)
		}
	}, nil
}
