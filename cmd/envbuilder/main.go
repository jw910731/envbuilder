package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/coder/coder/codersdk/agentsdk"
	"github.com/coder/envbuilder"
	"github.com/spf13/cobra"

	// *Never* remove this. Certificates are not bundled as part
	// of the container, so this is necessary for all connections
	// to not be insecure.
	_ "github.com/breml/rootcerts"
)

func main() {
	root := &cobra.Command{
		Use: "envbuilder",
		RunE: func(cmd *cobra.Command, args []string) error {
			options := envbuilder.SystemOptions(os.Getenv)

			var sendLogs func(log agentsdk.StartupLog)
			agentURL := os.Getenv("CODER_AGENT_URL")
			agentToken := os.Getenv("CODER_AGENT_TOKEN")
			if agentToken != "" {
				if agentURL == "" {
					return errors.New("CODER_AGENT_URL must be set if CODER_AGENT_TOKEN is set")
				}
				parsed, err := url.Parse(agentURL)
				if err != nil {
					return err
				}
				client := agentsdk.New(parsed)
				client.SetSessionToken(agentToken)
				client.SDK.HTTPClient = &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							InsecureSkipVerify: options.Insecure,
						},
					},
				}
				sendLogs, err = envbuilder.SendLogsToCoder(cmd.Context(), client, func(format string, args ...any) {
					fmt.Fprintf(cmd.ErrOrStderr(), format, args...)
				})
				if err != nil {
					return err
				}
			}

			options.Logger = func(format string, args ...interface{}) {
				output := fmt.Sprintf(format, args...)
				fmt.Fprintln(cmd.ErrOrStderr(), output)
				if sendLogs != nil {
					sendLogs(agentsdk.StartupLog{
						CreatedAt: time.Now(),
						Output:    output,
					})
				}
			}
			return envbuilder.Run(cmd.Context(), options)
		},
	}
	err := root.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
}