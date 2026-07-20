package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"
)

var (
	errGatewayReservationConflict = errors.New("gateway_reservation_conflict")
	errGatewayBalanceConflict     = errors.New("gateway_balance_conflict")
	errGatewayUserUnavailable     = errors.New("gateway_user_unavailable")
)

type gatewaySettlement struct {
	Request    GatewayRequest
	Usage      UsageRecord
	OldBalance int64
	NewBalance int64
	Ledgers    []TokenLedger
	APIKey     *APIKey
	ClientKey  *ClientAccessKey
	Session    *GatewaySession
	Audits     []AuditLog
	NextID     int64
}

func (s *Store) loadGatewayReplay(ctx context.Context, request GatewayRequest) (codexResponsesResult, bool, error) {
	if request.Status != gatewayCompleted || request.UpdatedAt.Before(time.Now().UTC().Add(-gatewayReplayBodyRetention)) {
		return codexResponsesResult{}, false, nil
	}
	if s.db == nil {
		if request.ResultBody == "" {
			return codexResponsesResult{}, false, nil
		}
		status := request.UpstreamStatus
		if status == 0 {
			status = http.StatusOK
		}
		return codexResponsesResult{
			Status:      status,
			Header:      decodeCodexResponseHeaders(request.ResultHeaders),
			Body:        []byte(request.ResultBody),
			ContentType: request.ResultType,
		}, true, nil
	}

	var result codexResponsesResult
	var headers string
	err := s.db.QueryRowContext(ctx, `SELECT upstream_status,result_body,result_type,result_headers FROM idempotency_records WHERE user_id=$1 AND request_id=$2 AND status=$3 AND result_body<>'' AND updated_at>$4`, request.UserID, request.RequestID, gatewayCompleted, time.Now().UTC().Add(-gatewayReplayBodyRetention)).Scan(&result.Status, &result.Body, &result.ContentType, &headers)
	if errors.Is(err, sql.ErrNoRows) {
		return codexResponsesResult{}, false, nil
	}
	if err != nil {
		return codexResponsesResult{}, false, err
	}
	if result.Status == 0 {
		result.Status = http.StatusOK
	}
	result.Header = decodeCodexResponseHeaders(headers)
	return result, true, nil
}

func (s *Store) pruneGatewayPersistence(ctx context.Context, now time.Time) (int64, int64, error) {
	if s.db == nil {
		return 0, 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	cleared, err := tx.ExecContext(ctx, `UPDATE idempotency_records SET result_body='' WHERE status=$1 AND result_body<>'' AND updated_at<=$2`, gatewayCompleted, now.Add(-gatewayReplayBodyRetention))
	if err != nil {
		return 0, 0, err
	}
	deleted, err := tx.ExecContext(ctx, `DELETE FROM idempotency_records WHERE status<>$1 AND updated_at<=$2`, gatewayReserved, now.Add(-gatewayIdempotencyRetention))
	if err != nil {
		return 0, 0, err
	}
	clearedCount, err := cleared.RowsAffected()
	if err != nil {
		return 0, 0, err
	}
	deletedCount, err := deleted.RowsAffected()
	if err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return clearedCount, deletedCount, nil
}

func (s *Store) saveGatewayReservation(ctx context.Context, request GatewayRequest) error {
	if s.db == nil {
		return s.save()
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	var balance int64
	if err := tx.QueryRowContext(ctx, `SELECT status, token_balance FROM users WHERE id=$1 FOR UPDATE`, request.UserID).Scan(&status, &balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errGatewayUserUnavailable
		}
		return err
	}
	if status != statusActive {
		return errGatewayUserUnavailable
	}
	rows, err := tx.QueryContext(ctx, `SELECT reserved_tokens FROM idempotency_records WHERE user_id=$1 AND status=$2 AND request_id<>$3 FOR UPDATE`, request.UserID, gatewayReserved, request.RequestID)
	if err != nil {
		return err
	}
	var held int64
	for rows.Next() {
		var reserved int64
		if err := rows.Scan(&reserved); err != nil {
			_ = rows.Close()
			return err
		}
		held += reserved
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if balance-held < request.ReservedTokens {
		return errGatewayBalanceConflict
	}
	result, err := tx.ExecContext(ctx, `
INSERT INTO idempotency_records (id, user_id, request_id, status, reserved_tokens, charged_tokens, usage_record_id, upstream_status, error, result_text, result_body, result_type, result_headers, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,0,'',0,'','','','','',$6,$7)
ON CONFLICT (user_id, request_id) DO UPDATE SET
  status=EXCLUDED.status, reserved_tokens=EXCLUDED.reserved_tokens, charged_tokens=0,
  usage_record_id='', upstream_status=0, error='', result_text='', result_body='', result_type='', result_headers='', updated_at=EXCLUDED.updated_at
WHERE idempotency_records.status=$8`,
		request.ID, request.UserID, request.RequestID, gatewayReserved, request.ReservedTokens, request.CreatedAt, request.UpdatedAt, gatewayFailed)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errGatewayReservationConflict
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO app_meta (key,value) VALUES ('next_id',$1) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, strconv.FormatInt(s.state.NextID, 10)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) saveGatewayFailure(ctx context.Context, request GatewayRequest, audits []AuditLog) error {
	if s.db == nil {
		return s.save()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE idempotency_records SET status=$1,reserved_tokens=0,upstream_status=$2,error=$3,updated_at=$4 WHERE user_id=$5 AND request_id=$6`, request.Status, request.UpstreamStatus, request.Error, request.UpdatedAt, request.UserID, request.RequestID)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return errGatewayReservationConflict
	}
	if err := insertGatewayAudits(ctx, tx, audits); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO app_meta (key,value) VALUES ('next_id',$1) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, strconv.FormatInt(s.state.NextID, 10)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) saveGatewaySettlement(ctx context.Context, settlement gatewaySettlement) error {
	if s.db == nil {
		return s.save()
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	request := settlement.Request
	result, err := tx.ExecContext(ctx, `UPDATE idempotency_records SET status=$1,reserved_tokens=0,charged_tokens=$2,usage_record_id=$3,upstream_status=$4,error='',result_text=$5,result_body=$6,result_type=$7,result_headers=$8,updated_at=$9 WHERE user_id=$10 AND request_id=$11 AND status=$12`, request.Status, request.ChargedTokens, request.UsageRecordID, request.UpstreamStatus, request.ResultText, request.ResultBody, request.ResultType, request.ResultHeaders, request.UpdatedAt, request.UserID, request.RequestID, gatewayReserved)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return errGatewayReservationConflict
	}
	if settlement.OldBalance != settlement.NewBalance {
		result, err = tx.ExecContext(ctx, `UPDATE users SET token_balance=$1,updated_at=$2 WHERE id=$3 AND token_balance=$4`, settlement.NewBalance, request.UpdatedAt, request.UserID, settlement.OldBalance)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			if err != nil {
				return err
			}
			return errGatewayBalanceConflict
		}
	}
	rec := settlement.Usage
	if _, err := tx.ExecContext(ctx, `INSERT INTO usage_records (id,user_id,upstream_account_id,api_key_id,client_access_key_id,session_id,model,input_tokens,cached_input_tokens,output_tokens,total_tokens,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, rec.ID, rec.UserID, rec.UpstreamAccountID, rec.APIKeyID, rec.ClientAccessKeyID, rec.SessionID, rec.Model, rec.InputTokens, rec.CachedInputTokens, rec.OutputTokens, rec.TotalTokens, rec.CreatedAt); err != nil {
		return err
	}
	for _, ledger := range settlement.Ledgers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO token_ledgers (id,user_id,type,delta_tokens,balance_after,source,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, ledger.ID, ledger.UserID, ledger.Type, ledger.DeltaTokens, ledger.BalanceAfter, ledger.Source, ledger.CreatedAt); err != nil {
			return err
		}
	}
	if settlement.APIKey != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE api_keys SET last_used_at=$1,updated_at=$2 WHERE id=$3`, settlement.APIKey.LastUsedAt, settlement.APIKey.UpdatedAt, settlement.APIKey.ID); err != nil {
			return err
		}
	}
	if settlement.ClientKey != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE client_access_keys SET last_used_at=$1,updated_at=$2 WHERE id=$3`, settlement.ClientKey.LastUsedAt, settlement.ClientKey.UpdatedAt, settlement.ClientKey.ID); err != nil {
			return err
		}
	}
	if settlement.Session != nil {
		session := settlement.Session
		if _, err := tx.ExecContext(ctx, `INSERT INTO gateway_session_routes (user_id,session_key,upstream_account_id,expires_at,updated_at) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (user_id,session_key) DO UPDATE SET upstream_account_id=EXCLUDED.upstream_account_id,expires_at=EXCLUDED.expires_at,updated_at=EXCLUDED.updated_at`, session.UserID, session.SessionKey, session.UpstreamAccountID, session.ExpiresAt, session.UpdatedAt); err != nil {
			return err
		}
	}
	if err := insertGatewayAudits(ctx, tx, settlement.Audits); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO app_meta (key,value) VALUES ('next_id',$1) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, strconv.FormatInt(settlement.NextID, 10)); err != nil {
		return err
	}
	return tx.Commit()
}

func insertGatewayAudits(ctx context.Context, tx *sql.Tx, audits []AuditLog) error {
	for _, audit := range audits {
		if _, err := tx.ExecContext(ctx, `INSERT INTO audit_logs (id,actor_id,actor_role,action,target_id,detail,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (id) DO NOTHING`, audit.ID, audit.ActorID, audit.ActorRole, audit.Action, audit.TargetID, audit.Detail, audit.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}
