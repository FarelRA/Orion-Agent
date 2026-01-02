package command

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.mau.fi/whatsmeow/types"

	"orion-agent/internal/data/store"
	"orion-agent/internal/service/send"
)

// Command is the interface all commands must implement.
type Command interface {
	Name() string
	Description() string
	Usage() string
	RequiresAdmin() bool
	Execute(ctx context.Context, args []string, execCtx *ExecutionContext) (string, error)
}

// ExecutionContext provides context for command execution.
type ExecutionContext struct {
	ChatJID   types.JID
	SenderJID types.JID
	IsAdmin   bool
}

// Registry manages command registration and execution.
type Registry struct {
	commands    map[string]Command
	settings    *store.SettingsStore
	sendService *send.SendService
	mu          sync.RWMutex
}

// NewRegistry creates a new command registry.
func NewRegistry(settings *store.SettingsStore, sendService *send.SendService) *Registry {
	return &Registry{
		commands:    make(map[string]Command),
		settings:    settings,
		sendService: sendService,
	}
}

// Register adds a command to the registry.
func (r *Registry) Register(cmd Command) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands[cmd.Name()] = cmd
}

// Get retrieves a command by name.
func (r *Registry) Get(name string) (Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cmd, ok := r.commands[name]
	return cmd, ok
}

// List returns all registered command names.
func (r *Registry) List() []Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cmds := make([]Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// IsCommand checks if text is a command.
func (r *Registry) IsCommand(text string) bool {
	prefix := r.settings.GetCommandPrefix()
	return strings.HasPrefix(strings.TrimSpace(text), prefix)
}

// Parse extracts command name and args from text.
func (r *Registry) Parse(text string) (name string, args []string) {
	prefix := r.settings.GetCommandPrefix()
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, prefix) {
		return "", nil
	}

	text = strings.TrimPrefix(text, prefix)
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", nil
	}

	return strings.ToLower(parts[0]), parts[1:]
}

// Execute runs a command and sends the response.
func (r *Registry) Execute(ctx context.Context, text string, chatJID, senderJID types.JID) error {
	name, args := r.Parse(text)
	if name == "" {
		return nil
	}

	cmd, ok := r.Get(name)
	if !ok {
		return r.sendResponse(ctx, chatJID, fmt.Sprintf("Unknown command: %s. Use /help for available commands.", name))
	}

	isAdmin := r.settings.IsAdmin(senderJID.String())
	execCtx := &ExecutionContext{
		ChatJID:   chatJID,
		SenderJID: senderJID,
		IsAdmin:   isAdmin,
	}

	if cmd.RequiresAdmin() && !isAdmin {
		return r.sendResponse(ctx, chatJID, "This command requires admin privileges.")
	}

	response, err := cmd.Execute(ctx, args, execCtx)
	if err != nil {
		return r.sendResponse(ctx, chatJID, fmt.Sprintf("Error: %s", err.Error()))
	}

	if response != "" {
		return r.sendResponse(ctx, chatJID, response)
	}

	return nil
}

func (r *Registry) sendResponse(ctx context.Context, chatJID types.JID, text string) error {
	_, err := r.sendService.Send(ctx, chatJID, send.Text(text))
	return err
}

// RegisterBuiltinCommands registers all built-in commands.
func (r *Registry) RegisterBuiltinCommands() {
	r.Register(&HelpCommand{registry: r})
	r.Register(&StatusCommand{settings: r.settings})
	r.Register(&EnableCommand{settings: r.settings})
	r.Register(&DisableCommand{settings: r.settings})
	r.Register(&WhitelistCommand{settings: r.settings})
	r.Register(&BlacklistCommand{settings: r.settings})
	r.Register(&AdminCommand{settings: r.settings})
}

// HelpCommand shows available commands.
type HelpCommand struct {
	registry *Registry
}

func (c *HelpCommand) Name() string        { return "help" }
func (c *HelpCommand) Description() string { return "Show available commands" }
func (c *HelpCommand) Usage() string       { return "/help [command]" }
func (c *HelpCommand) RequiresAdmin() bool { return false }

func (c *HelpCommand) Execute(ctx context.Context, args []string, execCtx *ExecutionContext) (string, error) {
	if len(args) > 0 {
		cmd, ok := c.registry.Get(args[0])
		if !ok {
			return fmt.Sprintf("Unknown command: %s", args[0]), nil
		}
		return fmt.Sprintf("*%s*\n%s\nUsage: %s\nAdmin only: %v",
			cmd.Name(), cmd.Description(), cmd.Usage(), cmd.RequiresAdmin()), nil
	}

	var sb strings.Builder
	sb.WriteString("*Available Commands:*\n")
	for _, cmd := range c.registry.List() {
		if cmd.RequiresAdmin() && !execCtx.IsAdmin {
			continue
		}
		sb.WriteString(fmt.Sprintf("• /%s - %s\n", cmd.Name(), cmd.Description()))
	}
	return sb.String(), nil
}

// StatusCommand shows AI status.
type StatusCommand struct {
	settings *store.SettingsStore
}

func (c *StatusCommand) Name() string        { return "status" }
func (c *StatusCommand) Description() string { return "Show AI agent status" }
func (c *StatusCommand) Usage() string       { return "/status" }
func (c *StatusCommand) RequiresAdmin() bool { return false }

func (c *StatusCommand) Execute(ctx context.Context, args []string, execCtx *ExecutionContext) (string, error) {
	enabled := c.settings.GetAIEnabled()
	status := "disabled"
	if enabled {
		status = "enabled"
	}
	return fmt.Sprintf("AI Agent Status: *%s*", status), nil
}

// EnableCommand enables the AI.
type EnableCommand struct {
	settings *store.SettingsStore
}

func (c *EnableCommand) Name() string        { return "enable" }
func (c *EnableCommand) Description() string { return "Enable AI agent" }
func (c *EnableCommand) Usage() string       { return "/enable" }
func (c *EnableCommand) RequiresAdmin() bool { return true }

func (c *EnableCommand) Execute(ctx context.Context, args []string, execCtx *ExecutionContext) (string, error) {
	if err := c.settings.SetAIEnabled(true); err != nil {
		return "", err
	}
	return "AI Agent enabled.", nil
}

// DisableCommand disables the AI.
type DisableCommand struct {
	settings *store.SettingsStore
}

func (c *DisableCommand) Name() string        { return "disable" }
func (c *DisableCommand) Description() string { return "Disable AI agent" }
func (c *DisableCommand) Usage() string       { return "/disable" }
func (c *DisableCommand) RequiresAdmin() bool { return true }

func (c *DisableCommand) Execute(ctx context.Context, args []string, execCtx *ExecutionContext) (string, error) {
	if err := c.settings.SetAIEnabled(false); err != nil {
		return "", err
	}
	return "AI Agent disabled.", nil
}

// WhitelistCommand manages the whitelist.
type WhitelistCommand struct {
	settings *store.SettingsStore
}

func (c *WhitelistCommand) Name() string        { return "whitelist" }
func (c *WhitelistCommand) Description() string { return "Manage whitelist (add/remove/list)" }
func (c *WhitelistCommand) Usage() string       { return "/whitelist [add|remove|list] [jid]" }
func (c *WhitelistCommand) RequiresAdmin() bool { return true }

func (c *WhitelistCommand) Execute(ctx context.Context, args []string, execCtx *ExecutionContext) (string, error) {
	if len(args) == 0 {
		args = []string{"list"}
	}

	whitelist := c.settings.GetWhitelist()

	switch args[0] {
	case "list":
		if len(whitelist) == 0 {
			return "Whitelist is empty (all allowed).", nil
		}
		return fmt.Sprintf("Whitelist:\n%s", strings.Join(whitelist, "\n")), nil

	case "add":
		if len(args) < 2 {
			return "Usage: /whitelist add <jid>", nil
		}
		whitelist = append(whitelist, args[1])
		if err := c.settings.SetWhitelist(whitelist); err != nil {
			return "", err
		}
		return fmt.Sprintf("Added %s to whitelist.", args[1]), nil

	case "remove":
		if len(args) < 2 {
			return "Usage: /whitelist remove <jid>", nil
		}
		newList := make([]string, 0)
		for _, jid := range whitelist {
			if jid != args[1] {
				newList = append(newList, jid)
			}
		}
		if err := c.settings.SetWhitelist(newList); err != nil {
			return "", err
		}
		return fmt.Sprintf("Removed %s from whitelist.", args[1]), nil

	default:
		return "Usage: /whitelist [add|remove|list] [jid]", nil
	}
}

// BlacklistCommand manages the blacklist.
type BlacklistCommand struct {
	settings *store.SettingsStore
}

func (c *BlacklistCommand) Name() string        { return "blacklist" }
func (c *BlacklistCommand) Description() string { return "Manage blacklist (add/remove/list)" }
func (c *BlacklistCommand) Usage() string       { return "/blacklist [add|remove|list] [jid]" }
func (c *BlacklistCommand) RequiresAdmin() bool { return true }

func (c *BlacklistCommand) Execute(ctx context.Context, args []string, execCtx *ExecutionContext) (string, error) {
	if len(args) == 0 {
		args = []string{"list"}
	}

	blacklist := c.settings.GetBlacklist()

	switch args[0] {
	case "list":
		if len(blacklist) == 0 {
			return "Blacklist is empty.", nil
		}
		return fmt.Sprintf("Blacklist:\n%s", strings.Join(blacklist, "\n")), nil

	case "add":
		if len(args) < 2 {
			return "Usage: /blacklist add <jid>", nil
		}
		blacklist = append(blacklist, args[1])
		if err := c.settings.SetBlacklist(blacklist); err != nil {
			return "", err
		}
		return fmt.Sprintf("Added %s to blacklist.", args[1]), nil

	case "remove":
		if len(args) < 2 {
			return "Usage: /blacklist remove <jid>", nil
		}
		newList := make([]string, 0)
		for _, jid := range blacklist {
			if jid != args[1] {
				newList = append(newList, jid)
			}
		}
		if err := c.settings.SetBlacklist(newList); err != nil {
			return "", err
		}
		return fmt.Sprintf("Removed %s from blacklist.", args[1]), nil

	default:
		return "Usage: /blacklist [add|remove|list] [jid]", nil
	}
}

// AdminCommand manages admins.
type AdminCommand struct {
	settings *store.SettingsStore
}

func (c *AdminCommand) Name() string        { return "admin" }
func (c *AdminCommand) Description() string { return "Manage admin list (add/remove/list)" }
func (c *AdminCommand) Usage() string       { return "/admin [add|remove|list] [jid]" }
func (c *AdminCommand) RequiresAdmin() bool { return true }

func (c *AdminCommand) Execute(ctx context.Context, args []string, execCtx *ExecutionContext) (string, error) {
	if len(args) == 0 {
		args = []string{"list"}
	}

	admins := c.settings.GetAdmins()

	switch args[0] {
	case "list":
		if len(admins) == 0 {
			return "No admins configured.", nil
		}
		return fmt.Sprintf("Admins:\n%s", strings.Join(admins, "\n")), nil

	case "add":
		if len(args) < 2 {
			return "Usage: /admin add <jid>", nil
		}
		admins = append(admins, args[1])
		if err := c.settings.SetAdmins(admins); err != nil {
			return "", err
		}
		return fmt.Sprintf("Added %s as admin.", args[1]), nil

	case "remove":
		if len(args) < 2 {
			return "Usage: /admin remove <jid>", nil
		}
		newList := make([]string, 0)
		for _, jid := range admins {
			if jid != args[1] {
				newList = append(newList, jid)
			}
		}
		if err := c.settings.SetAdmins(newList); err != nil {
			return "", err
		}
		return fmt.Sprintf("Removed %s from admins.", args[1]), nil

	default:
		return "Usage: /admin [add|remove|list] [jid]", nil
	}
}
