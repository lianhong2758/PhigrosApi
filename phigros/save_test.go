package phigros_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lianhong2758/PhigrosAPI/phigros"
)

func loadSampleSave(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "data", "gamesave", "nkyjch88ydrg4js83bea9jyiw.zip")
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("sample save missing: %v", err)
	}
	if err := phigros.LoadDifficult(filepath.Join("..", "difficulty.tsv")); err != nil {
		t.Fatalf("load difficulty: %v", err)
	}
	return root
}

func TestParseSave(t *testing.T) {
	path := loadSampleSave(t)

	save, err := phigros.ParseSave(path)
	if err != nil {
		t.Fatalf("ParseSave failed: %v", err)
	}

	if save.GameKey.Version != 2 {
		t.Fatalf("unexpected gameKey version: %d", save.GameKey.Version)
	}
	if save.GameProgress.Version != 3 {
		t.Fatalf("unexpected gameProgress version: %d", save.GameProgress.Version)
	}
	if len(save.GameRecord) == 0 {
		t.Fatal("gameRecord should not be empty")
	}
	if len(save.GameRecord.Score()) == 0 {
		t.Fatal("score list should not be empty")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	path := loadSampleSave(t)

	save, err := phigros.ParseSave(path)
	if err != nil {
		t.Fatalf("ParseSave failed: %v", err)
	}
	fmt.Println(save.GameKey)
	data, err := phigros.MarshalSave(save)
	if err != nil {
		t.Fatalf("MarshalSave failed: %v", err)
	}

	parsed, err := phigros.ParseSaveBytes(data)
	if err != nil {
		t.Fatalf("ParseSaveBytes failed: %v", err)
	}

	if !reflect.DeepEqual(save, parsed) {
		t.Fatal("save round trip mismatch")
	}
}
