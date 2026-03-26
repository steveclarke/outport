package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for outport.

Bash:
  $ source <(outport completion bash)

  # To load completions for every new session, add to ~/.bashrc:
  eval "$(outport completion bash)"

Zsh:
  # To load completions in your current session:
  $ source <(outport completion zsh)

  # To load completions for every new session, add AFTER compinit in ~/.zshrc:
  eval "$(outport completion zsh)"

Fish:
  $ outport completion fish | source

  # To load completions for each session, execute once:
  $ outport completion fish > ~/.config/fish/completions/outport.fish
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletionV2(os.Stdout, true)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		default:
			return fmt.Errorf("unknown shell: %s", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
