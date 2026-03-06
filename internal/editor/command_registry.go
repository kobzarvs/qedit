package editor

import "strings"

type CommandHandler func(*Editor, []string) bool

type CommandDefinition struct {
	Names   []string
	Entries []CommandInfo
	Handle  CommandHandler
}

type commandRegistry struct {
	handlers     map[string]CommandDefinition
	autocomplete []CommandInfo
}

func newCommandRegistry() commandRegistry {
	return commandRegistry{
		handlers: make(map[string]CommandDefinition),
	}
}

func (r *commandRegistry) Register(def CommandDefinition) {
	if r.handlers == nil {
		r.handlers = make(map[string]CommandDefinition)
	}
	for _, name := range def.Names {
		r.handlers[name] = def
	}
	r.autocomplete = append(r.autocomplete, def.Entries...)
}

func (r *commandRegistry) Lookup(name string) (CommandDefinition, bool) {
	def, ok := r.handlers[name]
	return def, ok
}

func (r *commandRegistry) Filter(prefix string) []CommandInfo {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return append([]CommandInfo(nil), r.autocomplete...)
	}
	var result []CommandInfo
	for _, cmd := range r.autocomplete {
		if strings.HasPrefix(cmd.Name, prefix) {
			result = append(result, cmd)
		}
	}
	return result
}

func (e *Editor) RegisterCommand(def CommandDefinition) {
	e.commands.Register(def)
}

func (e *Editor) filterCommands(prefix string) []CommandInfo {
	return e.commands.Filter(prefix)
}

func (e *Editor) registerBuiltInCommands() {
	e.RegisterCommand(CommandDefinition{
		Names: []string{"w"},
		Entries: []CommandInfo{
			{Name: "w", Description: "write file", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeWriteCommand(args, false)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"e", "edit"},
		Entries: []CommandInfo{
			{Name: "e", Description: "reload file", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeReloadCommand(args, false)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"e!", "edit!"},
		Entries: []CommandInfo{
			{Name: "e!", Description: "reload file (discard changes)", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeReloadCommand(args, true)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"q"},
		Entries: []CommandInfo{
			{Name: "q", Description: "quit", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeQuitCommand(false)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"q!"},
		Entries: []CommandInfo{
			{Name: "q!", Description: "force quit", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeQuitCommand(true)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"wq", "x"},
		Entries: []CommandInfo{
			{Name: "wq", Description: "write and quit", Group: CmdGroupFile},
			{Name: "x", Description: "write and quit", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeWriteCommand(args, true)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"ln"},
		Entries: []CommandInfo{
			{Name: "ln", Description: "line numbers", Group: CmdGroupView},
			{Name: "ln off", Description: "disable line numbers", Group: CmdGroupView},
			{Name: "ln abs", Description: "absolute line numbers", Group: CmdGroupView},
			{Name: "ln rel", Description: "relative line numbers", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeLineNumberCommand(args)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"fmt"},
		Entries: []CommandInfo{
			{Name: "fmt", Description: "format code", Group: CmdGroupEdit},
		},
		Handle: func(ed *Editor, args []string) bool {
			if err := ed.queueFormatRequest(); err != nil {
				ed.setStatus(err.Error())
				return false
			}
			return false
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"sidebar"},
		Entries: []CommandInfo{
			{Name: "sidebar", Description: "toggle sidebar", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			ed.toggleSidebar()
			return false
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"tree"},
		Entries: []CommandInfo{
			{Name: "tree", Description: "open file tree", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeTreeCommand(args)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"sidew"},
		Entries: []CommandInfo{
			{Name: "sidew", Description: "set sidebar width", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeSidebarWidthCommand(args)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"sidebar-focus"},
		Entries: []CommandInfo{
			{Name: "sidebar-focus", Description: "toggle sidebar focus", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			ed.toggleSidebarFocus()
			return false
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"focus-editor"},
		Entries: []CommandInfo{
			{Name: "focus-editor", Description: "focus editor", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			ed.focusEditor()
			return false
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"autoreload", "auto-reload", "auto-reload-on-changes"},
		Entries: []CommandInfo{
			{Name: "auto-reload-on-changes", Description: "auto reload on external changes", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeAutoReloadCommand(args)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"merge"},
		Entries: []CommandInfo{
			{Name: "merge", Description: "merge mode", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			if len(args) > 0 {
				ed.setStatus("merge takes no arguments")
				return false
			}
			return ed.enterMergeMode()
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"worktree", "worktrees"},
		Entries: []CommandInfo{
			{Name: "worktree", Description: "open worktree menu", Group: CmdGroupGit},
			{Name: "worktree list", Description: "list worktrees", Group: CmdGroupGit},
			{Name: "worktree new <name>", Description: "create worktree from current branch", Group: CmdGroupGit},
			{Name: "worktree switch <name>", Description: "switch to worktree", Group: CmdGroupGit},
			{Name: "worktree remove <name>", Description: "remove worktree", Group: CmdGroupGit},
			{Name: "worktree refresh", Description: "refresh worktree list", Group: CmdGroupGit},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.handleWorktreeCommand(args)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"bn"},
		Entries: []CommandInfo{
			{Name: "bn", Description: "next buffer", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			ed.gotoNextBuffer()
			return false
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"bp"},
		Entries: []CommandInfo{
			{Name: "bp", Description: "previous buffer", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			ed.gotoPrevBuffer()
			return false
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"bc"},
		Entries: []CommandInfo{
			{Name: "bc", Description: "close buffer", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			ed.closeCurrentBuffer(false)
			return false
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"bc!"},
		Entries: []CommandInfo{
			{Name: "bc!", Description: "close buffer (force)", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			ed.closeCurrentBuffer(true)
			return false
		},
	})
}
