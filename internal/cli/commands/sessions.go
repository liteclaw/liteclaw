package commands

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/liteclaw/liteclaw/internal/gateway"
	"github.com/spf13/cobra"
)

func NewSessionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List stored conversation sessions",
		Long:  `Manage and view chat sessions stored by the agent.`,
	}

	cmd.AddCommand(newSessionsListCommand())

	return cmd
}

func newSessionsListCommand() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List sessions",
		Example: `  liteclaw sessions list --limit 10`,
		Run: func(cmd *cobra.Command, args []string) {
			dataDir := os.Getenv("LITECLAW_DATA_DIR")
			sm := gateway.NewSessionManager(dataDir)
			sessions := sm.ListSessions()

			// Sort by updated desc
			sort.Slice(sessions, func(i, j int) bool {
				return sessions[i].UpdatedAt > sessions[j].UpdatedAt
			})

			if limit > 0 && len(sessions) > limit {
				sessions = sessions[:limit]
			}

			if len(sessions) == 0 {
				cmd.Println("No sessions found.")
				return
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "Key\tSessionID\tChannel\tRecency")

			for _, s := range sessions {
				key := s.Key
				sid := s.SessionID
				if len(sid) > 8 {
					sid = sid[:8]
				}

				// Calculate nice recency
				ts := time.UnixMilli(s.UpdatedAt)
				recency := time.Since(ts).Round(time.Second).String() + " ago"
				if time.Since(ts) < time.Minute {
					recency = "just now"
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", key, sid, s.Channel, recency)
			}
			w.Flush()
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Limit number of sessions shown")

	return cmd
}
