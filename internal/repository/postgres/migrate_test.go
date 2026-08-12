package postgres

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/gliedabrennung/sedna/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrate_ConcurrentCallersAreSerialised(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set, skipping postgres integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	const racers = 6
	errs := make([]error, racers)
	var wg sync.WaitGroup
	for i := range racers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = Migrate(ctx, pool, migrations.SQL)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("caller %d failed: %v", i, err)
		}
	}

	var duplicates int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM (
			SELECT version FROM schema_migrations GROUP BY version HAVING count(*) > 1
		) AS d`).Scan(&duplicates)
	if err != nil {
		t.Fatalf("count duplicates: %v", err)
	}
	if duplicates != 0 {
		t.Errorf("expected each migration to be recorded once, found %d duplicated", duplicates)
	}
}
