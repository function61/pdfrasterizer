package main

import (
	"context"
	"fmt"
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

		imageType := "image/png"
		// doesn't use the features offered by HTTP's "Accept" mechanism, but that's pretty
		// hard to parse (and this feature doesn't warrant vetting for negotiation library)
		if r.Header.Get("Accept") == "image/jpeg" {
			imageType = "image/jpeg"
		}

		// needs to be unique for each request. using named pipe because Ghostscript pollutes
		// stdout with log messages, and therefore it doesn't support writing to work output there
		outputPath, cleanup, err := randomFifoName()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer cleanup()

		gsDevice, err := func() (string, error) {
			switch imageType {
			case "image/jpeg":
				return "jpeg", nil
			case "image/png":
				return "png16m", nil
			default:
				return "", fmt.Errorf("unsupported requested image type: %s", imageType)
			}
		}()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ghostscript := exec.Command(
			"./gs",
			"-dNOPAUSE",
			"-dBATCH",
			"-o", outputPath,
			"-dUseCropBox",
			"-r300",               // internal rendering DPI
			"-dDownScaleFactor=3", // 300/3 = 100 DPI
			"-sDEVICE="+gsDevice,
			"-dJPEGQ=95", // does not error when given to PNG also
			"-",          // = take from stdin
		)
		ghostscript.Stdin = r.Body
		defer r.Body.Close()

		w.Header().Set("Content-Type", imageType)

		// Ghostscript and reading from FIFO need to happen concurrently, b/c writing to
		// the FIFO blocks until the bytes are consumed
		outputDone := runSimpleTaskAsync(func() error {
			ghostscriptOutput, err := os.Open(outputPath)
			if err != nil {
				return err
			}
			defer ghostscriptOutput.Close()

			// send image to client
			_, err = io.Copy(w, ghostscriptOutput)
			return err
		})

		ghostscriptDone := runSimpleTaskAsync(func() error {
			return ghostscript.Run()
		})

		if err := <-ghostscriptDone; err != nil {
			log.Printf("ghostscript run: %v", err)
		}

		if err := <-outputDone; err != nil {
			log.Printf("output err: %v", err)
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

	rasterized, err := client.RasterizeToPng(ctx, input)
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

func runSimpleTaskAsync(fn func() error) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- fn()
	}()
	return errCh
}
