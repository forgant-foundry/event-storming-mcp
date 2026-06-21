package cmd

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"

	mcpserver "github.com/forgant-foundry/event-storming-mcp/internal/mcp"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server over stdio",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := mcpserver.New()
		if err := server.ServeStdio(s); err != nil {
			fmt.Fprintln(os.Stderr, "server error:", err)
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
