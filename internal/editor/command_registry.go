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
		Names: []string{"r", "read"},
		Entries: []CommandInfo{
			{Name: "r <file>", Description: "read file below cursor", Group: CmdGroupFile},
			{Name: "r !<cmd>", Description: "read shell output below cursor", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeReadCommand(args)
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
		Names: []string{"set"},
		Entries: []CommandInfo{
			{Name: "set", Description: "show editor options", Group: CmdGroupView},
			{Name: "set ic", Description: "enable ignorecase search", Group: CmdGroupView},
			{Name: "set noic", Description: "disable ignorecase search", Group: CmdGroupView},
			{Name: "set invic", Description: "toggle ignorecase search", Group: CmdGroupView},
			{Name: "set hls is", Description: "enable search highlighting and incremental search", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeSetCommand(args)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"nohlsearch", "noh"},
		Entries: []CommandInfo{
			{Name: "nohlsearch", Description: "clear highlighted search matches", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			if len(args) > 0 {
				ed.setStatus("usage: nohlsearch")
				return false
			}
			return ed.executeNoHLSearchCommand()
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
		Names: []string{"vs", "vsplit"},
		Entries: []CommandInfo{
			{Name: "vs [file]", Description: "open vertical split", Group: CmdGroupView},
			{Name: "vsplit [file]", Description: "open vertical split", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeSplitCommand(args, editorWindowVertical)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"hs", "hsplit"},
		Entries: []CommandInfo{
			{Name: "hs [file]", Description: "open horizontal split", Group: CmdGroupView},
			{Name: "hsplit [file]", Description: "open horizontal split", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeSplitCommand(args, editorWindowHorizontal)
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
		Names: []string{"profile"},
		Entries: []CommandInfo{
			{Name: "profile", Description: "show current behavior profile", Group: CmdGroupView},
			{Name: "profile basic", Description: "switch to basic profile", Group: CmdGroupView},
			{Name: "profile helix", Description: "switch to helix profile", Group: CmdGroupView},
			{Name: "profile vim", Description: "switch to vim profile", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.executeProfileCommand(args)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"tutor", "Tutor"},
		Entries: []CommandInfo{
			{Name: "tutor", Description: "open current profile tutorial", Group: CmdGroupView},
			{Name: "tutor helix", Description: "open Helix tutorial", Group: CmdGroupView},
			{Name: "tutor vim", Description: "open Vim tutorial", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			if len(args) > 1 {
				ed.setStatus("usage: tutor [vim|helix]")
				return false
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return ed.openTutor(name)
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
		Names: []string{"buffers", "ls", "files"},
		Entries: []CommandInfo{
			{Name: "buffers", Description: "list open buffers", Group: CmdGroupFile},
			{Name: "ls", Description: "list open buffers", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			ed.showBufferList()
			return false
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"b", "buffer"},
		Entries: []CommandInfo{
			{Name: "b <target>", Description: "switch buffer", Group: CmdGroupFile},
			{Name: "buffer <target>", Description: "switch buffer", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			if len(args) == 0 {
				return ed.switchToBufferTarget("")
			}
			return ed.switchToBufferTarget(strings.Join(args, " "))
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"b#"},
		Entries: []CommandInfo{
			{Name: "b#", Description: "switch to alternate buffer", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			if len(args) > 0 {
				ed.setStatus("usage: b#")
				return false
			}
			return ed.switchToBufferTarget("#")
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"bn", "bnext"},
		Entries: []CommandInfo{
			{Name: "bn", Description: "next buffer", Group: CmdGroupFile},
			{Name: "bnext", Description: "next buffer", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			if len(args) > 0 {
				ed.setStatus("usage: bnext")
				return false
			}
			ed.gotoNextBuffer()
			return false
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"bp", "bprev", "bprevious"},
		Entries: []CommandInfo{
			{Name: "bp", Description: "previous buffer", Group: CmdGroupFile},
			{Name: "bprevious", Description: "previous buffer", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			if len(args) > 0 {
				ed.setStatus("usage: bprevious")
				return false
			}
			ed.gotoPrevBuffer()
			return false
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"bd", "bdelete", "bc"},
		Entries: []CommandInfo{
			{Name: "bd", Description: "delete buffer", Group: CmdGroupFile},
			{Name: "bdelete", Description: "delete buffer", Group: CmdGroupFile},
			{Name: "bc", Description: "close buffer", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.closeBufferTarget(strings.Join(args, " "), false)
		},
	})
	e.RegisterCommand(CommandDefinition{
		Names: []string{"bd!", "bdelete!", "bc!"},
		Entries: []CommandInfo{
			{Name: "bd!", Description: "delete buffer (force)", Group: CmdGroupFile},
			{Name: "bdelete!", Description: "delete buffer (force)", Group: CmdGroupFile},
			{Name: "bc!", Description: "close buffer (force)", Group: CmdGroupFile},
		},
		Handle: func(ed *Editor, args []string) bool {
			return ed.closeBufferTarget(strings.Join(args, " "), true)
		},
	})
}
