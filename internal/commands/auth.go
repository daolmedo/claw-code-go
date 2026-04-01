package commands

import (
	"claw-code-go/internal/auth"
	"fmt"
	"strings"
)

// RegisterAuthCommands adds the /auth command group to the registry.
func RegisterAuthCommands(r *Registry) {
	r.Register(Command{
		Name:        "auth",
		Description: "Authentication: /auth login | logout | status",
		Handler:     handleAuthCommand,
	})
}

func handleAuthCommand(args string, _ interface{}) error {
	parts := strings.Fields(args)
	sub := "status"
	if len(parts) > 0 {
		sub = parts[0]
	}

	switch sub {
	case "login":
		return cmdAuthLogin()
	case "logout":
		return cmdAuthLogout()
	case "status":
		return cmdAuthStatus()
	default:
		fmt.Printf("Unknown auth subcommand %q. Usage: /auth login | logout | status\n", sub)
		return nil
	}
}

func cmdAuthLogin() error {
	fmt.Println("Starting OAuth login...")
	td, err := auth.StartOAuthFlow()
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if err := auth.SaveTokens(td); err != nil {
		return fmt.Errorf("save tokens: %w", err)
	}
	fmt.Println("Login successful. Token saved to ~/.claw-code/auth.json")
	return nil
}

func cmdAuthLogout() error {
	if err := auth.ClearTokens(); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	fmt.Println("Logged out. Stored tokens cleared.")
	return nil
}

func cmdAuthStatus() error {
	s := auth.GetStatus()
	fmt.Printf("Auth status:\n")
	fmt.Printf("  Authenticated : %v\n", s.Authenticated)
	fmt.Printf("  Method        : %s\n", s.Method)
	if s.Method == "oauth" && !s.ExpiresAt.IsZero() {
		fmt.Printf("  Token expires : %s\n", s.ExpiresAt.Format("2006-01-02 15:04:05 MST"))
		fmt.Printf("  Has refresh   : %v\n", s.HasRefresh)
	}
	return nil
}
