package llmInteraction

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ LLmInteractionRepository = (*PostgresLlmInteractionRepo)(nil)

type LLmInteractionRepository interface {
	SaveInteraction(ctx context.Context, interaction types.LlmInteraction) error
}

type PostgresLlmInteractionRepo struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewPostgresLlmInteractionRepo(pgxpool *pgxpool.Pool, logger *slog.Logger) *PostgresLlmInteractionRepo {
	return &PostgresLlmInteractionRepo{
		logger: logger,
		pgpool: pgxpool,
	}
}

func (r *PostgresLlmInteractionRepo) SaveInteraction(ctx context.Context, interaction types.LlmInteraction) error {
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
        INSERT INTO llm_interactions (
            user_id, prompt, response_text, model_used, latency_ms
        ) VALUES ($1, $2, $3, $4, $5)
    `
	_, err = tx.Exec(ctx, query,
		interaction.UserID, interaction.Prompt, interaction.ResponseText,
		interaction.ModelUsed, interaction.LatencyMs,
	)

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return err
}
