package editor

import "strings"

const (
	BehaviorProfileBasic = "basic"
	BehaviorProfileHelix = "helix"
	BehaviorProfileVim   = "vim"
)

type BehaviorProfile struct {
	Name      string
	HandleKey func(*Editor, EventKey) bool
}

type behaviorProfileRegistry struct {
	profiles map[string]BehaviorProfile
}

func newBehaviorProfileRegistry() behaviorProfileRegistry {
	return behaviorProfileRegistry{
		profiles: make(map[string]BehaviorProfile),
	}
}

func normalizeBehaviorProfileName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return BehaviorProfileHelix
	}
	return name
}

func (r *behaviorProfileRegistry) Register(profile BehaviorProfile) {
	if r.profiles == nil {
		r.profiles = make(map[string]BehaviorProfile)
	}
	profile.Name = normalizeBehaviorProfileName(profile.Name)
	if profile.Name == "" {
		return
	}
	r.profiles[profile.Name] = profile
}

func (r *behaviorProfileRegistry) Lookup(name string) (BehaviorProfile, bool) {
	profile, ok := r.profiles[normalizeBehaviorProfileName(name)]
	return profile, ok
}

func (e *Editor) RegisterBehaviorProfile(profile BehaviorProfile) {
	e.behaviorProfiles.Register(profile)
}

func (e *Editor) registerBuiltInBehaviorProfiles() {
	e.RegisterBehaviorProfile(BehaviorProfile{
		Name: BehaviorProfileBasic,
		HandleKey: func(ed *Editor, ev EventKey) bool {
			return ed.handleBasicProfileKey(ev)
		},
	})
	e.RegisterBehaviorProfile(BehaviorProfile{
		Name: BehaviorProfileHelix,
		HandleKey: func(ed *Editor, ev EventKey) bool {
			return ed.handleHelixProfileKey(ev)
		},
	})
	e.RegisterBehaviorProfile(BehaviorProfile{
		Name: BehaviorProfileVim,
		HandleKey: func(ed *Editor, ev EventKey) bool {
			return ed.handleVimProfileKey(ev)
		},
	})
}

func (e *Editor) SetBehaviorProfile(name string) bool {
	name = normalizeBehaviorProfileName(name)
	if _, ok := e.behaviorProfiles.Lookup(name); !ok {
		return false
	}
	e.profile.name = name
	e.profile.vim = vimProfileState{}
	if name == BehaviorProfileBasic && e.mode == ModeNormal {
		e.mode = ModeInsert
	}
	if name == BehaviorProfileVim && e.mode == ModeInsert && e.document.filename == "" && e.Content() == "" {
		e.mode = ModeNormal
	}
	return true
}

func (e *Editor) BehaviorProfile() string {
	name := normalizeBehaviorProfileName(e.profile.name)
	if _, ok := e.behaviorProfiles.Lookup(name); ok {
		return name
	}
	return BehaviorProfileHelix
}

func (e *Editor) handleProfileKey(ev EventKey) bool {
	if profile, ok := e.behaviorProfiles.Lookup(e.profile.name); ok && profile.HandleKey != nil {
		return profile.HandleKey(e, ev)
	}
	if profile, ok := e.behaviorProfiles.Lookup(BehaviorProfileHelix); ok && profile.HandleKey != nil {
		e.profile.name = BehaviorProfileHelix
		return profile.HandleKey(e, ev)
	}
	return false
}

func (e *Editor) handleBasicProfileKey(ev EventKey) bool {
	switch e.mode {
	case ModeCommand:
		return e.handleCommand(ev)
	case ModeBranchPicker:
		return e.handleBranchPicker(ev)
	case ModeSearch:
		return e.handleSearch(ev)
	case ModeMerge:
		return e.handleMerge(ev)
	default:
		if e.mode != ModeInsert {
			e.mode = ModeInsert
		}
		return e.handleInsert(ev)
	}
}

func (e *Editor) handleHelixProfileKey(ev EventKey) bool {
	switch e.mode {
	case ModeInsert:
		return e.handleInsert(ev)
	case ModeCommand:
		return e.handleCommand(ev)
	case ModeBranchPicker:
		return e.handleBranchPicker(ev)
	case ModeSearch:
		return e.handleSearch(ev)
	case ModeMerge:
		return e.handleMerge(ev)
	default:
		return e.handleNormal(ev)
	}
}

func (e *Editor) currentModeLabel() string {
	switch e.mode {
	case ModeCommand:
		return "COMMAND"
	case ModeBranchPicker:
		return "BRANCHES"
	case ModeSearch:
		return "SEARCH"
	case ModeMerge:
		if e.gitDiffPreviewActive() {
			return "DIFF"
		}
		if e.mergeReviewActive() {
			return "REVIEW"
		}
		return "MERGE"
	}

	switch e.BehaviorProfile() {
	case BehaviorProfileBasic:
		if e.hugeFileActive() {
			return e.hugeFileModeLabel()
		}
		return "BASIC"
	case BehaviorProfileVim:
		if e.hugeFileActive() {
			return e.hugeFileModeLabel()
		}
		if e.profile.vim.visual {
			return "VISUAL"
		}
		if e.mode == ModeInsert {
			return "INSERT"
		}
		return "NORMAL"
	default:
		if e.hugeFileActive() {
			return e.hugeFileModeLabel()
		}
		if e.mode == ModeInsert {
			return "INSERT"
		}
		return "NORMAL"
	}
}

func (e *Editor) currentCursorStyle() CursorStyle {
	if e.mode == ModeSearch || e.mode == ModeCommand {
		return CursorStyleSteadyBar
	}
	switch e.BehaviorProfile() {
	case BehaviorProfileBasic:
		return CursorStyleSteadyBar
	case BehaviorProfileVim:
		if e.mode == ModeInsert {
			return CursorStyleSteadyBar
		}
		return CursorStyleSteadyBlock
	default:
		if e.mode == ModeInsert {
			return CursorStyleSteadyBar
		}
		return CursorStyleSteadyBlock
	}
}
