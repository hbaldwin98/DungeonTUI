package gitsync

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/storage"
)

func TestPushThenPullCarriesTheWorkspace(t *testing.T) {
	remote := bareRemote(t)
	a := testClient(t, filepath.Join(t.TempDir(), "a"), remote)
	b := testClient(t, filepath.Join(t.TempDir(), "b"), remote)
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Push(sampleSnapshot()); err != nil {
		t.Fatal(err)
	}
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	got, result, err := b.Pull(Snapshot{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Commit == "" {
		t.Fatal("expected a commit hash")
	}
	if len(got.Workspace.Records) != 1 || got.Workspace.Records[0].Title != "Mira" {
		t.Fatalf("pulled %#v", got.Workspace.Records)
	}
	if len(got.Adventures) != 1 || got.Adventures[0].ID != "src-5e-crown" {
		t.Fatalf("pulled adventures %#v", got.Adventures)
	}
}

func TestPullMergesDistinctRecordsFromTwoMachines(t *testing.T) {
	remote := bareRemote(t)
	a := testClient(t, filepath.Join(t.TempDir(), "a"), remote)
	b := testClient(t, filepath.Join(t.TempDir(), "b"), remote)
	base := sampleSnapshot()
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Push(base); err != nil {
		t.Fatal(err)
	}
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.Pull(Snapshot{}, false); err != nil {
		t.Fatal(err)
	}

	vale := domain.Record{
		ID: "npc-vale", Type: domain.NPC, Title: "Vale", Authority: domain.Canon,
		Scope: base.Workspace.Scope, Body: "Wary of the party.",
	}
	holt := domain.Record{
		ID: "npc-holt", Type: domain.NPC, Title: "Holt", Authority: domain.Draft,
		Scope: base.Workspace.Scope, Body: "Keeps the books.",
	}
	if _, err := a.Push(withRecord(base, vale)); err != nil {
		t.Fatal(err)
	}
	got, _, err := b.Pull(withRecord(base, holt), false)
	if err != nil {
		t.Fatal(err)
	}
	titles := recordTitles(got)
	if !strings.Contains(titles, "Mira") || !strings.Contains(titles, "Vale") || !strings.Contains(titles, "Holt") {
		t.Fatalf("merged records should keep all three, got %s", titles)
	}
}

func TestPullConflictLeavesLocalSnapshotUnchanged(t *testing.T) {
	remote := bareRemote(t)
	a := testClient(t, filepath.Join(t.TempDir(), "a"), remote)
	b := testClient(t, filepath.Join(t.TempDir(), "b"), remote)
	base := sampleSnapshot()
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Push(base); err != nil {
		t.Fatal(err)
	}
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.Pull(Snapshot{}, false); err != nil {
		t.Fatal(err)
	}

	aEdit := base
	aEdit.Workspace.Records = []domain.Record{base.Workspace.Records[0]}
	aEdit.Workspace.Records[0].Body = "rewritten on machine A"
	bEdit := base
	bEdit.Workspace.Records = []domain.Record{base.Workspace.Records[0]}
	bEdit.Workspace.Records[0].Body = "rewritten on machine B"
	if _, err := a.Push(aEdit); err != nil {
		t.Fatal(err)
	}
	_, _, err := b.Pull(bEdit, false)
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected ConflictError, got %v", err)
	}
	held, err := Read(b.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if held.Workspace.Records[0].Body != "rewritten on machine B" {
		t.Fatalf("conflict must not apply the other machine: %#v", held.Workspace.Records[0])
	}
}

func TestPushRefusesWhenRemoteIsAhead(t *testing.T) {
	remote := bareRemote(t)
	a := testClient(t, filepath.Join(t.TempDir(), "a"), remote)
	b := testClient(t, filepath.Join(t.TempDir(), "b"), remote)
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Push(sampleSnapshot()); err != nil {
		t.Fatal(err)
	}
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.Pull(Snapshot{}, false); err != nil {
		t.Fatal(err)
	}
	extra := withRecord(sampleSnapshot(), domain.Record{
		ID: "npc-vale", Type: domain.NPC, Title: "Vale", Authority: domain.Canon,
		Scope: sampleSnapshot().Workspace.Scope, Body: "Wary.",
	})
	if _, err := a.Push(extra); err != nil {
		t.Fatal(err)
	}
	_, err := b.Push(sampleSnapshot())
	if err == nil || !strings.Contains(err.Error(), "pull first") {
		t.Fatalf("expected refuse until pull, got %v", err)
	}
}

func TestPushDeletionTravelsToTheOtherMachine(t *testing.T) {
	remote := bareRemote(t)
	a := testClient(t, filepath.Join(t.TempDir(), "a"), remote)
	b := testClient(t, filepath.Join(t.TempDir(), "b"), remote)
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Push(sampleSnapshot()); err != nil {
		t.Fatal(err)
	}
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.Pull(Snapshot{}, false); err != nil {
		t.Fatal(err)
	}
	cleared := sampleSnapshot()
	cleared.Workspace.Records = nil
	if _, err := a.Push(cleared); err != nil {
		t.Fatal(err)
	}
	got, _, err := b.Pull(Snapshot{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Workspace.Records) != 0 {
		t.Fatalf("deleted record still present after pull: %#v", got.Workspace.Records)
	}
}

func TestForcePullTakesRemoteAndDropsLocalEdits(t *testing.T) {
	remote := bareRemote(t)
	a := testClient(t, filepath.Join(t.TempDir(), "a"), remote)
	b := testClient(t, filepath.Join(t.TempDir(), "b"), remote)
	base := sampleSnapshot()
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Push(base); err != nil {
		t.Fatal(err)
	}
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.Pull(Snapshot{}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Push(withRecord(base, domain.Record{
		ID: "npc-vale", Type: domain.NPC, Title: "Vale", Authority: domain.Canon,
		Scope: base.Workspace.Scope, Body: "Wary.",
	})); err != nil {
		t.Fatal(err)
	}
	got, _, err := b.Pull(withRecord(base, domain.Record{
		ID: "npc-holt", Type: domain.NPC, Title: "Holt", Authority: domain.Draft,
		Scope: base.Workspace.Scope, Body: "Keeps the books.",
	}), true)
	if err != nil {
		t.Fatal(err)
	}
	titles := recordTitles(got)
	if !strings.Contains(titles, "Vale") {
		t.Fatalf("force pull should take remote, got %s", titles)
	}
	if strings.Contains(titles, "Holt") {
		t.Fatalf("force pull must drop unpushed local records, got %s", titles)
	}
}

func TestPullKeepsUnpushedAdventureCache(t *testing.T) {
	remote := bareRemote(t)
	a := testClient(t, filepath.Join(t.TempDir(), "a"), remote)
	b := testClient(t, filepath.Join(t.TempDir(), "b"), remote)
	base := sampleSnapshot()
	base.Adventures = nil
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Push(base); err != nil {
		t.Fatal(err)
	}
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.Pull(Snapshot{}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Push(withRecord(base, domain.Record{
		ID: "npc-vale", Type: domain.NPC, Title: "Vale", Authority: domain.Canon,
		Scope: base.Workspace.Scope, Body: "Wary.",
	})); err != nil {
		t.Fatal(err)
	}
	local := base
	local.Adventures = []storage.Blob{{ID: "src-local", Data: []byte(`{"ID":"src-local"}`)}}
	got, _, err := b.Pull(local, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Adventures) != 1 || got.Adventures[0].ID != "src-local" {
		t.Fatalf("unpushed adventure cache was dropped: %#v", got.Adventures)
	}
	if !strings.Contains(recordTitles(got), "Vale") {
		t.Fatalf("remote record missing after merge: %s", recordTitles(got))
	}
}

func TestApplyReplacesWorkspaceIncludingDeletes(t *testing.T) {
	store := storage.NewSQLite(filepath.Join(t.TempDir(), "workspace.sqlite"))
	defer store.Close()
	if err := Apply(store, sampleSnapshot()); err != nil {
		t.Fatal(err)
	}
	cleared := sampleSnapshot()
	cleared.Workspace.Records = nil
	cleared.Adventures = nil
	if err := Apply(store, cleared); err != nil {
		t.Fatal(err)
	}
	got, err := SnapshotFrom(store)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Workspace.Records) != 0 {
		t.Fatalf("apply should delete records absent from the snapshot: %#v", got.Workspace.Records)
	}
	if len(got.Adventures) != 0 {
		t.Fatalf("apply should delete adventures absent from the snapshot: %#v", got.Adventures)
	}
}

func TestApplyRoundTripsThroughSqlite(t *testing.T) {
	store := storage.NewSQLite(filepath.Join(t.TempDir(), "workspace.sqlite"))
	defer store.Close()
	want := sampleSnapshot()
	if err := Apply(store, want); err != nil {
		t.Fatal(err)
	}
	got, err := SnapshotFrom(store)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Workspace.Records) != 1 || got.Workspace.Records[0].Title != "Mira" {
		t.Fatalf("sqlite snapshot %#v", got.Workspace.Records)
	}
	if len(got.Adventures) != 1 || got.Adventures[0].ID != "src-5e-crown" {
		t.Fatalf("sqlite adventures %#v", got.Adventures)
	}
}

func TestGitReportsUnavailableBinary(t *testing.T) {
	g := Git{Binary: filepath.Join(t.TempDir(), "absent-git")}
	if g.Available() {
		t.Fatal("absent binary must not report available")
	}
	_, err := g.Run("", "status")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestSaveAndLoadSyncConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "sync.json")
	want := Config{Remote: "git@example.com:me/campaigns.git", Branch: "main", Dir: "/tmp/sync"}
	if err := SaveConfig(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("config %#v", got)
	}
}

func TestLoadConfigMissingIsNotExist(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "missing.json"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadConfig error = %v", err)
	}
}

func withRecord(snap Snapshot, rec domain.Record) Snapshot {
	snap.Workspace.Records = append(append([]domain.Record{}, snap.Workspace.Records...), rec)
	return snap
}

func recordTitles(snap Snapshot) string {
	var titles []string
	for _, rec := range snap.Workspace.Records {
		titles = append(titles, rec.Title)
	}
	return strings.Join(titles, ",")
}

func testClient(t *testing.T, dir, remote string) Client {
	t.Helper()
	return Client{
		Dir:    dir,
		Remote: remote,
		Branch: "main",
		Git:    testGit(t),
	}
}

func testGit(t *testing.T) Git {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for sync tests")
	}
	home := t.TempDir()
	return Git{
		Env: gitTestEnv(home),
		Extra: []string{
			"-c", "user.name=DungeonTest",
			"-c", "user.email=dungeon@test",
			"-c", "commit.gpgsign=false",
			"-c", "init.defaultBranch=main",
		},
	}
}

func gitTestEnv(home string) []string {
	return []string{
		"HOME=" + home,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=DungeonTest",
		"GIT_AUTHOR_EMAIL=dungeon@test",
		"GIT_COMMITTER_NAME=DungeonTest",
		"GIT_COMMITTER_EMAIL=dungeon@test",
		"PATH=" + os.Getenv("PATH"),
	}
}

func bareRemote(t *testing.T) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "remote.git")
	g := testGit(t)
	if _, err := g.Run("", "init", "--bare", "-b", "main", remote); err != nil {
		t.Fatal(err)
	}
	return remote
}
