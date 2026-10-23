package cities

import (
	"conexiuni-cluj/config"
	"conexiuni-cluj/database"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestDevelopmentUsesOnlyTheDevelopmentKeys(t *testing.T) {
	t.Setenv("TRANZY_API_KEY_CLUJ", "city")
	t.Setenv("TRANZY_API_KEY_CLUJ_LEARNING", "city-learning")
	t.Setenv("TRANZY_API_KEY_DEVELOPMENT", "")
	t.Setenv("TRANZY_API_KEY_DEVELOPMENT_LEARNING", "")
	feedsIn := func(environment string) (Feeds, error) {
		dir := t.TempDir()
		open := func() (*database.Store, error) {
			return database.Open(filepath.Join(dir, "cluj.db"), "", 5)
		}
		env := NewEnv("cluj", &config.Config{Environment: environment, TranzyRateLimit: time.Second}, open)
		t.Cleanup(func() {
			if store, err := env.Store(); err == nil {
				store.Close()
			}
		})
		return env.Tranzy(2)
	}

	if _, err := feedsIn("development"); !errors.Is(err, ErrNoKey) {
		t.Fatalf("development took the city's key: %v", err)
	}
	t.Setenv("TRANZY_API_KEY_DEVELOPMENT", "dev")
	if f, err := feedsIn("development"); err != nil || f.Learning != nil {
		t.Fatalf("development key alone: learning %v, err %v", f.Learning, err)
	}
	t.Setenv("TRANZY_API_KEY_DEVELOPMENT_LEARNING", "dev-learning")
	if f, err := feedsIn("development"); err != nil || f.Learning == nil {
		t.Fatalf("development learning key: learning %v, err %v", f.Learning, err)
	}
	if f, err := feedsIn("production"); err != nil || f.Learning == nil {
		t.Fatalf("production: learning %v, err %v", f.Learning, err)
	}
}
