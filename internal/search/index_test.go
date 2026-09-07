package search

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

func TestOpenPathPersistsAndReopens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "search.sqlite")
	docs := DocumentsFromWorkspace(domain.Workspace{
		Records: []domain.Record{{
			ID: "npc-vale", Type: domain.NPC, Title: "Captain Vale",
			Authority: domain.Canon, Scope: testScope, Body: "Wary of the party.",
		}},
	})
	first, err := OpenPath(path, docs)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	if !hasKind(first.Find(campaignFilter("vale")), KindRecord, "npc-vale") {
		t.Fatal("expected vale in new index")
	}
	first.Close()
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("expected persisted search.sqlite, err=%v", err)
	}

	second, err := OpenPath(path, docs)
	if err != nil {
		t.Fatalf("reopen index: %v", err)
	}
	defer second.Close()
	if !hasKind(second.Find(campaignFilter("wary")), KindRecord, "npc-vale") {
		t.Fatal("expected body hit after reopen")
	}
}

func TestOpenPathRebuildsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "search.sqlite")
	if err := os.WriteFile(path, []byte("this is not a sqlite database"), 0o644); err != nil {
		t.Fatal(err)
	}
	docs := DocumentsFromWorkspace(domain.Workspace{
		Records: []domain.Record{{
			ID: "npc-vale", Type: domain.NPC, Title: "Captain Vale",
			Authority: domain.Canon, Scope: testScope,
		}},
	})
	svc, err := OpenPath(path, docs)
	if err != nil {
		t.Fatalf("corrupt index should rebuild, err=%v", err)
	}
	defer svc.Close()
	if !hasKind(svc.Find(campaignFilter("cptvl")), KindRecord, "npc-vale") {
		t.Fatal("expected fuzzy hit after corrupt rebuild")
	}
}

func TestOpenPathFallsBackWhenFileUnwritable(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	docs := DocumentsFromWorkspace(domain.Workspace{
		Records: []domain.Record{{
			ID: "npc-vale", Type: domain.NPC, Title: "Captain Vale",
			Authority: domain.Canon, Scope: testScope,
		}},
	})
	svc, err := OpenPath(filepath.Join(blocked, "search.sqlite"), docs)
	if err == nil {
		t.Fatal("expected index path error")
	}
	defer svc.Close()
	if !hasKind(svc.Find(campaignFilter("vale")), KindRecord, "npc-vale") {
		t.Fatal("memory fallback should still search")
	}
}

func TestFindAcceptsFTSOperatorJunk(t *testing.T) {
	svc := FromWorkspace(domain.Workspace{
		Records: []domain.Record{{
			ID: "npc-vale", Type: domain.NPC, Title: "Captain Vale",
			Authority: domain.Canon, Scope: testScope,
		}},
	})
	defer svc.Close()
	svc.Find(campaignFilter(`AND OR NOT * ^ "`))
	svc.Find(campaignFilter(strings.Repeat("a", 400)))
}

func TestIndexReplaceAndFindRace(t *testing.T) {
	ws := domain.Workspace{
		Records: []domain.Record{{
			ID: "npc-vale", Type: domain.NPC, Title: "Captain Vale",
			Authority: domain.Canon, Scope: testScope, Body: "Greywatch captain.",
		}},
	}
	svc := FromWorkspace(ws)
	defer svc.Close()
	docs := DocumentsFromWorkspace(ws)

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				_ = svc.Find(campaignFilter("vale"))
				_ = svc.Find(campaignFilter("greywatch"))
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 20; j++ {
			if err := svc.index.Replace(docs); err != nil {
				t.Errorf("replace: %v", err)
			}
		}
	}()
	wg.Wait()
}
