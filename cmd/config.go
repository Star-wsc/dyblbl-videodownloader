package cmd

import (
	"fmt"

	"github.com/Star-wsc/dyblbl-videodownloader/internal/config"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `View and manage application configuration.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set configuration value",
	Long: `Set a configuration value. Available keys:
  - download-dir: Download directory path
  - proxy: HTTP proxy URL
  - bilibili-cookie: Bilibili cookie string
  - douyin-cookie: Douyin cookie string`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg := config.GetConfig()

	fmt.Println("Current Configuration:")
	fmt.Printf("  Download Directory: %s\n", cfg.DownloadDir)
	fmt.Printf("  Proxy: %s\n", cfg.Proxy)
	fmt.Printf("  Max Concurrent: %d\n", cfg.MaxConcurrent)
	fmt.Printf("  Speed Limit: %d KB/s\n", cfg.SpeedLimit)
	fmt.Printf("  File Template: %s\n", cfg.FileTemplate)
	fmt.Printf("  Auto Classify: %v\n", cfg.AutoClassify)
	fmt.Printf("  Bilibili Cookie: %s\n", maskString(cfg.BilibiliCookie))
	fmt.Printf("  Douyin Cookie: %s\n", maskString(cfg.DouyinCookie))

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]
	cfg := config.GetConfig()

	switch key {
	case "download-dir":
		if err := cfg.SetDownloadDir(value); err != nil {
			return fmt.Errorf("failed to set download directory: %w", err)
		}
		fmt.Printf("Download directory set to: %s\n", value)
	case "proxy":
		if err := cfg.SetProxy(value); err != nil {
			return fmt.Errorf("failed to set proxy: %w", err)
		}
		fmt.Printf("Proxy set to: %s\n", value)
	case "bilibili-cookie":
		if err := cfg.SetBilibiliCookie(value); err != nil {
			return fmt.Errorf("failed to set bilibili cookie: %w", err)
		}
		fmt.Println("Bilibili cookie set successfully")
	case "douyin-cookie":
		if err := cfg.SetDouyinCookie(value); err != nil {
			return fmt.Errorf("failed to set douyin cookie: %w", err)
		}
		fmt.Println("Douyin cookie set successfully")
	case "max-concurrent":
		val := 0
		fmt.Sscanf(value, "%d", &val)
		if val <= 0 {
			return fmt.Errorf("invalid value for max-concurrent: %s", value)
		}
		if err := cfg.SetMaxConcurrent(val); err != nil {
			return fmt.Errorf("failed to set max concurrent: %w", err)
		}
		fmt.Printf("Max concurrent set to: %d\n", val)
	case "speed-limit":
		val := 0
		fmt.Sscanf(value, "%d", &val)
		if val < 0 {
			return fmt.Errorf("invalid value for speed-limit: %s", value)
		}
		if err := cfg.SetSpeedLimit(val); err != nil {
			return fmt.Errorf("failed to set speed limit: %w", err)
		}
		fmt.Printf("Speed limit set to: %d KB/s\n", val)
	case "file-template":
		if err := cfg.SetFileTemplate(value); err != nil {
			return fmt.Errorf("failed to set file template: %w", err)
		}
		fmt.Printf("File template set to: %s\n", value)
	case "auto-classify":
		val := value == "true" || value == "1" || value == "yes"
		if err := cfg.SetAutoClassify(val); err != nil {
			return fmt.Errorf("failed to set auto classify: %w", err)
		}
		fmt.Printf("Auto classify set to: %v\n", val)
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}

	return nil
}

func maskString(s string) string {
	if s == "" {
		return "(not set)"
	}
	if len(s) > 10 {
		return s[:10] + "..."
	}
	return s + "..."
}
