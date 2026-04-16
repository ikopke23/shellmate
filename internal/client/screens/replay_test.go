package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func setupReplay(t *testing.T) *ReplayModel {
	t.Helper()
	m := NewReplayModel()
	if err := m.LoadPGN("1. e4 e5 2. Nf3 Nc6"); err != nil {
		t.Fatalf("LoadPGN: %v", err)
	}
	m.SetMeta("alice", "bob", time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC))
	return m
}

// initSaveInputs populates saveWhiteInput/saveBlackInput the same way handleBranchKeyMsg does.
func initSaveInputs(m *ReplayModel) {
	wi := textinput.New()
	wi.Placeholder = "White player name"
	wi.Focus()
	m.saveWhiteInput = wi
	bi := textinput.New()
	bi.Placeholder = "Black player name"
	m.saveBlackInput = bi
}

// --- Loading & metadata ---

func TestReplay_LoadPGN_PopulatesPositions(t *testing.T) {
	m := setupReplay(t)
	if len(m.positions) != 5 {
		t.Errorf("len(positions) = %d, want 5", len(m.positions))
	}
	if len(m.sanMoves) != 4 {
		t.Errorf("len(sanMoves) = %d, want 4", len(m.sanMoves))
	}
	if m.stepIdx != 0 {
		t.Errorf("stepIdx = %d, want 0", m.stepIdx)
	}
}

func TestReplay_LoadPGN_InvalidPGN_ReturnsError(t *testing.T) {
	m := NewReplayModel()
	if err := m.LoadPGN("1. e5"); err == nil {
		t.Error("expected error for invalid PGN, got nil")
	}
}

func TestReplay_SetMeta_StoresFields(t *testing.T) {
	m := NewReplayModel()
	when := time.Date(2026, 4, 16, 12, 34, 56, 0, time.UTC)
	m.SetMeta("alice", "bob", when)
	if m.white != "alice" {
		t.Errorf("white = %q, want alice", m.white)
	}
	if m.black != "bob" {
		t.Errorf("black = %q, want bob", m.black)
	}
	if !m.playedAt.Equal(when) {
		t.Errorf("playedAt = %v, want %v", m.playedAt, when)
	}
}

func TestReplay_SetBackScreen(t *testing.T) {
	m := NewReplayModel()
	if m.backScreen != ScreenHistory {
		t.Errorf("default backScreen = %v, want ScreenHistory", m.backScreen)
	}
	m.SetBackScreen(ScreenImportedGames)
	if m.backScreen != ScreenImportedGames {
		t.Errorf("backScreen = %v, want ScreenImportedGames", m.backScreen)
	}
}

// --- Navigation ---

func TestReplay_NavigateForward_AdvancesStep(t *testing.T) {
	m := setupReplay(t)
	m.replayNavigateForward()
	if m.stepIdx != 1 {
		t.Errorf("stepIdx = %d, want 1", m.stepIdx)
	}
}

func TestReplay_NavigateForward_AtEnd_StaysPut(t *testing.T) {
	m := setupReplay(t)
	m.stepIdx = len(m.moves)
	m.replayNavigateForward()
	if m.stepIdx != len(m.moves) {
		t.Errorf("stepIdx = %d, want %d", m.stepIdx, len(m.moves))
	}
}

func TestReplay_NavigateBack_DecrementsStep(t *testing.T) {
	m := setupReplay(t)
	m.stepIdx = 2
	m.replayNavigateBack()
	if m.stepIdx != 1 {
		t.Errorf("stepIdx = %d, want 1", m.stepIdx)
	}
}

func TestReplay_NavigateBack_AtStart_StaysPut(t *testing.T) {
	m := setupReplay(t)
	m.replayNavigateBack()
	if m.stepIdx != 0 {
		t.Errorf("stepIdx = %d, want 0", m.stepIdx)
	}
}

func TestReplay_HandleReplayKeyMsg_L_Forward(t *testing.T) {
	m := setupReplay(t)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.stepIdx != 1 {
		t.Errorf("stepIdx = %d, want 1", m.stepIdx)
	}
}

func TestReplay_HandleReplayKeyMsg_H_Back(t *testing.T) {
	m := setupReplay(t)
	m.stepIdx = 2
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.stepIdx != 1 {
		t.Errorf("stepIdx = %d, want 1", m.stepIdx)
	}
}

func TestReplay_HandleReplayKeyMsg_Q_ReturnsToBackScreen(t *testing.T) {
	m := setupReplay(t)
	m.SetBackScreen(ScreenImportedGames)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	sc, ok := msg.(ScreenChangeMsg)
	if !ok {
		t.Fatalf("msg type = %T, want ScreenChangeMsg", msg)
	}
	if sc.Screen != ScreenImportedGames {
		t.Errorf("Screen = %v, want ScreenImportedGames", sc.Screen)
	}
}

func TestReplay_HandleReplayKeyMsg_E_PopulatesClipboard(t *testing.T) {
	m := setupReplay(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.clipboardSeq == "" {
		t.Error("expected clipboardSeq to be populated")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	if _, ok := cmd().(clearClipboardMsg); !ok {
		t.Errorf("expected clearClipboardMsg, got different type")
	}
}

func TestReplay_HandleReplayKeyMsg_B_EntersBranchMode(t *testing.T) {
	m := setupReplay(t)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if !m.branchMode {
		t.Error("expected branchMode = true after 'b' key")
	}
}

// --- Branch mode ---

func TestReplay_EnterBranch_CopiesPosition(t *testing.T) {
	m := setupReplay(t)
	m.stepIdx = 2
	m.enterBranch()
	if !m.branchMode {
		t.Error("branchMode should be true")
	}
	if len(m.branchPositions) != 1 {
		t.Errorf("len(branchPositions) = %d, want 1", len(m.branchPositions))
	}
	if m.branchPointIdx != 2 {
		t.Errorf("branchPointIdx = %d, want 2", m.branchPointIdx)
	}
}

func TestReplay_ExitBranch_ClearsState(t *testing.T) {
	m := setupReplay(t)
	m.enterBranch()
	m.exitBranch()
	if m.branchMode {
		t.Error("branchMode should be false after exit")
	}
	if m.branchGame != nil {
		t.Error("branchGame should be nil after exit")
	}
	if m.branchSAN != nil {
		t.Error("branchSAN should be nil after exit")
	}
	if m.branchPositions != nil {
		t.Error("branchPositions should be nil after exit")
	}
	if m.input != nil {
		t.Error("input should be nil after exit")
	}
}

func TestReplay_ApplyBranchMove_AppendsMove(t *testing.T) {
	m := setupReplay(t)
	m.enterBranch()
	m.applyBranchMove("e4")
	if len(m.branchSAN) != 1 {
		t.Errorf("len(branchSAN) = %d, want 1", len(m.branchSAN))
	}
	if m.branchSAN[0] != "e4" {
		t.Errorf("branchSAN[0] = %q, want e4", m.branchSAN[0])
	}
	if len(m.branchPositions) != 2 {
		t.Errorf("len(branchPositions) = %d, want 2", len(m.branchPositions))
	}
}

func TestReplay_AtBranchTip_TrueAtEnd(t *testing.T) {
	m := setupReplay(t)
	m.enterBranch()
	m.applyBranchMove("e4")
	if !m.atBranchTip() {
		t.Error("expected atBranchTip to be true after applying move")
	}
}

func TestReplay_AtBranchTip_FalseInMiddle(t *testing.T) {
	m := setupReplay(t)
	m.enterBranch()
	m.applyBranchMove("e4")
	m.branchStepLeft()
	if m.atBranchTip() {
		t.Error("expected atBranchTip to be false after stepping back")
	}
}

func TestReplay_BranchStepLeft_Right_Bounds(t *testing.T) {
	m := setupReplay(t)
	m.stepIdx = 2
	m.enterBranch()
	m.applyBranchMove("d4")
	start := m.branchStepIdx
	m.branchStepRight()
	if m.branchStepIdx != start {
		t.Errorf("branchStepRight at tip should stay put: got %d, want %d", m.branchStepIdx, start)
	}
	m.branchStepLeft()
	if m.branchStepIdx != start-1 {
		t.Errorf("branchStepLeft should decrement: got %d, want %d", m.branchStepIdx, start-1)
	}
	for i := 0; i < 10; i++ {
		m.branchStepLeft()
	}
	if m.branchStepIdx != 0 {
		t.Errorf("branchStepLeft should clamp at 0, got %d", m.branchStepIdx)
	}
}

func TestReplay_BuildBranchPGN_IncludesMainAndBranch(t *testing.T) {
	m := setupReplay(t)
	m.stepIdx = 2
	m.enterBranch()
	m.applyBranchMove("d4")
	pgn := m.buildBranchPGN()
	if !strings.Contains(pgn, "e4") {
		t.Errorf("expected PGN to contain main move e4, got %q", pgn)
	}
	if !strings.Contains(pgn, "e5") {
		t.Errorf("expected PGN to contain main move e5, got %q", pgn)
	}
	if !strings.Contains(pgn, "d4") {
		t.Errorf("expected PGN to contain branch move d4, got %q", pgn)
	}
}

// --- Save dialog state machine ---

func TestReplay_SavePromptStep0_EnterAdvancesToStep1(t *testing.T) {
	m := setupReplay(t)
	initSaveInputs(m)
	m.savePromptActive = true
	m.saveStep = 0
	m.saveWhiteInput.SetValue("alice")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.saveStep != 1 {
		t.Errorf("saveStep = %d, want 1", m.saveStep)
	}
}

func TestReplay_SavePromptStep1_EnterEmitsCheckUsernames(t *testing.T) {
	m := setupReplay(t)
	initSaveInputs(m)
	m.savePromptActive = true
	m.saveStep = 1
	m.saveWhiteInput.SetValue("alice")
	m.saveBlackInput.SetValue("bob")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	cu, ok := msg.(CheckUsernamesActionMsg)
	if !ok {
		t.Fatalf("msg type = %T, want CheckUsernamesActionMsg", msg)
	}
	if cu.White != "alice" || cu.Black != "bob" {
		t.Errorf("got White=%q Black=%q, want alice/bob", cu.White, cu.Black)
	}
}

func TestReplay_SavePromptStep1_EscReturnsToStep0(t *testing.T) {
	m := setupReplay(t)
	initSaveInputs(m)
	m.savePromptActive = true
	m.saveStep = 1
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.saveStep != 0 {
		t.Errorf("saveStep = %d, want 0", m.saveStep)
	}
}

func TestReplay_SavePromptStep2_YEmitsDoSaveTrue(t *testing.T) {
	m := setupReplay(t)
	initSaveInputs(m)
	m.savePromptActive = true
	m.saveStep = 2
	m.saveWhiteInput.SetValue("alice")
	m.saveBlackInput.SetValue("bob")
	m.saveUnknownNames = []string{"alice"}
	m.saveConfirmIdx = 0
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	si, ok := msg.(SaveImportedActionMsg)
	if !ok {
		t.Fatalf("msg type = %T, want SaveImportedActionMsg", msg)
	}
	if !si.ForceCreate {
		t.Error("expected ForceCreate = true")
	}
}

func TestReplay_SavePromptStep2_NCancels(t *testing.T) {
	m := setupReplay(t)
	initSaveInputs(m)
	m.savePromptActive = true
	m.saveStep = 2
	m.saveUnknownNames = []string{"alice"}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.saveStep != 1 {
		t.Errorf("saveStep = %d, want 1", m.saveStep)
	}
	if m.saveUnknownNames != nil {
		t.Errorf("saveUnknownNames = %v, want nil", m.saveUnknownNames)
	}
}

func TestReplay_HandleUsernameCheckDoneMsg_NoUnknown_TriggersDoSave(t *testing.T) {
	m := setupReplay(t)
	initSaveInputs(m)
	m.saveWhiteInput.SetValue("alice")
	m.saveBlackInput.SetValue("bob")
	_, cmd := m.Update(UsernameCheckDoneMsg{Unknown: nil})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	si, ok := msg.(SaveImportedActionMsg)
	if !ok {
		t.Fatalf("msg type = %T, want SaveImportedActionMsg", msg)
	}
	if si.ForceCreate {
		t.Error("expected ForceCreate = false")
	}
}

func TestReplay_HandleUsernameCheckDoneMsg_HasUnknown_AdvancesToStep2(t *testing.T) {
	m := setupReplay(t)
	m.savePromptActive = true
	_, _ = m.Update(UsernameCheckDoneMsg{Unknown: []string{"alice"}})
	if m.saveStep != 2 {
		t.Errorf("saveStep = %d, want 2", m.saveStep)
	}
	if len(m.saveUnknownNames) != 1 || m.saveUnknownNames[0] != "alice" {
		t.Errorf("saveUnknownNames = %v, want [alice]", m.saveUnknownNames)
	}
}

func TestReplay_HandleSaveImportedDoneMsg_ClearsPrompt(t *testing.T) {
	m := setupReplay(t)
	m.savePromptActive = true
	_, _ = m.Update(SaveImportedDoneMsg{})
	if m.savePromptActive {
		t.Error("expected savePromptActive = false")
	}
	if m.saveMsg != "game saved" {
		t.Errorf("saveMsg = %q, want %q", m.saveMsg, "game saved")
	}
}
