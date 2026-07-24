package cmd

import (
	"fmt"

	"github.com/Rana718/dira/internal/wallpaper"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	wallGood = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
	wallBad  = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	wallInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

var setwallCmd = &cobra.Command{
	Use:   "set-wall",
	Short: "Set Hyprland wallpaper via file picker",
	Long:  `Opens a file picker, then applies the selected image as wallpaper on all monitors using hyprpaper.`,
	Example: `  dira set-wall        # opens zenity file picker
  dira set-wall --last # re-apply the last used wallpaper`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		last, _ := cmd.Flags().GetBool("last")

		var path string

		if last {
			path = wallpaper.LastWall()
			if path == "" {
				return fmt.Errorf("no previous wallpaper saved — run `dira set-wall` first")
			}
			fmt.Println(wallInfo.Render("re-applying: " + path))
		} else {
			var err error
			path, err = wallpaper.PickFile()
			if err != nil {
				if err.Error() == "cancelled" {
					fmt.Println(wallInfo.Render("cancelled."))
					return nil
				}
				return err
			}
		}

		fmt.Println(wallInfo.Render("applying: " + path))

		if err := wallpaper.Apply(path); err != nil {
			fmt.Println(wallBad.Render("✗ " + err.Error()))
			return err
		}

		fmt.Println(wallGood.Render("✓ wallpaper set"))
		return nil
	},
}

func init() {
	setwallCmd.Flags().Bool("last", false, "Re-apply the last used wallpaper")
}
