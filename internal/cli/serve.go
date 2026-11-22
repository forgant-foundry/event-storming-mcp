package cli

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"

	mcpserver "github.com/forgant-foundry/event-storming-mcp/internal/mcp"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start an MCP server over stdio: events (event storming) or stories (story mapping)",
}

var serveEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Serve event storming over stdio",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serveStdio(mcpserver.New())
	},
}

var serveStoriesCmd = &cobra.Command{
	Use:   "stories",
	Short: "Serve story mapping over stdio",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serveStdio(mcpserver.NewStories())
	},
}

func serveStdio(s *server.MCPServer) error {
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintln(os.Stderr, "server error:", err)
		return err
	}
	return nil
}

func init() {
	serveCmd.AddCommand(serveEventsCmd, serveStoriesCmd)
	rootCmd.AddCommand(serveCmd)
}
