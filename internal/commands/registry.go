package commands

import (
	"fmt"
	"strings"
)

// Command represents a slash command.
type Command struct {
	Name        string
	Description string
	Handler     func(args string, loop interface{}) error
}

// Registry holds all registered slash commands.
type Registry struct {
	commands map[string]Command
}

// NewRegistry creates a new command registry with built-in commands.
func NewRegistry() *Registry {
	r := &Registry{
		commands: make(map[string]Command),
	}

	r.registerBuiltins()
	return r
}

// Register adds a command to the registry.
func (r *Registry) Register(cmd Command) {
	name := strings.TrimPrefix(cmd.Name, "/")
	r.commands[name] = cmd
}

// Execute processes an input line. Returns (true, nil) if the command was handled,
// (false, nil) if not a command, or (true, err) if a command errored.
func (r *Registry) Execute(input string, loop interface{}) (bool, error) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return false, nil
	}

	// Split into command name and args
	parts := strings.SplitN(input[1:], " ", 2)
	name := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	cmd, ok := r.commands[name]
	if !ok {
		fmt.Printf("Unknown command: /%s. Type /help for available commands.\n", name)
		return true, nil
	}

	return true, cmd.Handler(args, loop)
}

// List returns all registered commands.
func (r *Registry) List() []Command {
	cmds := make([]Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// ErrExit is returned by /exit and /quit to signal the REPL should stop.
var ErrExit = fmt.Errorf("exit")

// registerBuiltins registers the built-in slash commands.
func (r *Registry) registerBuiltins() {
	r.Register(Command{
		Name:        "help",
		Description: "Show available commands",
		Handler: func(args string, loop interface{}) error {
			fmt.Println("Available commands:")
			for name, cmd := range r.commands {
				fmt.Printf("  /%s — %s\n", name, cmd.Description)
			}
			return nil
		},
	})

	r.Register(Command{
		Name:        "exit",
		Description: "Exit the REPL",
		Handler: func(args string, loop interface{}) error {
			return ErrExit
		},
	})

	r.Register(Command{
		Name:        "quit",
		Description: "Exit the REPL",
		Handler: func(args string, loop interface{}) error {
			return ErrExit
		},
	})

	r.Register(Command{
		Name:        "clear",
		Description: "Clear the conversation history",
		Handler: func(args string, loop interface{}) error {
			// Type assertion to access the conversation loop
			type sessionHolder interface {
				ClearSession()
			}
			if sh, ok := loop.(sessionHolder); ok {
				sh.ClearSession()
				fmt.Println("Conversation history cleared.")
			} else {
				fmt.Println("Cannot clear session: incompatible loop type.")
			}
			return nil
		},
	})

	r.Register(Command{
		Name:        "session-list",
		Description: "List saved sessions",
		Handler: func(args string, loop interface{}) error {
			type sessionLister interface {
				ListSessions() ([]string, error)
			}
			if sl, ok := loop.(sessionLister); ok {
				sessions, err := sl.ListSessions()
				if err != nil {
					return fmt.Errorf("list sessions: %w", err)
				}
				if len(sessions) == 0 {
					fmt.Println("No saved sessions.")
					return nil
				}
				fmt.Println("Saved sessions:")
				for _, s := range sessions {
					fmt.Printf("  %s\n", s)
				}
			} else {
				fmt.Println("Cannot list sessions: incompatible loop type.")
			}
			return nil
		},
	})
}
