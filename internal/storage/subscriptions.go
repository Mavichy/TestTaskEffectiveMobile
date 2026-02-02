package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("not found")

type Subscription struct {
	ID          string
	ServiceName string
	Price       int
	UserID      string
	StartDate   time.Time
	EndDate     *time.Time
}

type CreateSubscription struct {
	ServiceName string
	Price       int
	UserID      string
	StartDate   time.Time
	EndDate     *time.Time
}

type UpdateSubscription struct {
	ServiceName *string
	Price       *int
	UserID      *string
	StartDate   *time.Time
	EndDate     **time.Time
}

type ListFilter struct {
	UserID      *string
	ServiceName *string
	Limit       int
	Offset      int
}

type SubscriptionsRepo struct {
	db *DB
}

func NewSubscriptionsRepo(db *DB) *SubscriptionsRepo {
	return &SubscriptionsRepo{db: db}
}

func (r *SubscriptionsRepo) Create(ctx context.Context, in CreateSubscription) (Subscription, error) {
	var s Subscription
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO subscriptions(service_name, price, user_id, start_date, end_date)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, service_name, price, user_id, start_date, end_date
	`, in.ServiceName, in.Price, in.UserID, in.StartDate, in.EndDate).Scan(
		&s.ID, &s.ServiceName, &s.Price, &s.UserID, &s.StartDate, &s.EndDate,
	)
	return s, err
}

func (r *SubscriptionsRepo) Get(ctx context.Context, id string) (Subscription, error) {
	var s Subscription
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, service_name, price, user_id, start_date, end_date
		FROM subscriptions
		WHERE id=$1
	`, id).Scan(&s.ID, &s.ServiceName, &s.Price, &s.UserID, &s.StartDate, &s.EndDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return Subscription{}, ErrNotFound
	}
	return s, err
}

func (r *SubscriptionsRepo) Delete(ctx context.Context, id string) error {
	ct, err := r.db.Pool.Exec(ctx, `DELETE FROM subscriptions WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SubscriptionsRepo) Update(ctx context.Context, id string, in UpdateSubscription) (Subscription, error) {
	set := make([]string, 0, 5)
	args := make([]any, 0, 6)
	n := 1

	if in.ServiceName != nil {
		set = append(set, fmt.Sprintf("service_name=$%d", n))
		args = append(args, *in.ServiceName)
		n++
	}
	if in.Price != nil {
		set = append(set, fmt.Sprintf("price=$%d", n))
		args = append(args, *in.Price)
		n++
	}
	if in.UserID != nil {
		set = append(set, fmt.Sprintf("user_id=$%d", n))
		args = append(args, *in.UserID)
		n++
	}
	if in.StartDate != nil {
		set = append(set, fmt.Sprintf("start_date=$%d", n))
		args = append(args, *in.StartDate)
		n++
	}
	if in.EndDate != nil {
		set = append(set, fmt.Sprintf("end_date=$%d", n))
		args = append(args, *in.EndDate)
		n++
	}

	if len(set) == 0 {
		return r.Get(ctx, id)
	}

	args = append(args, id)

	q := fmt.Sprintf(`
		UPDATE subscriptions
		SET %s
		WHERE id=$%d
		RETURNING id, service_name, price, user_id, start_date, end_date
	`, strings.Join(set, ", "), n)

	var s Subscription
	err := r.db.Pool.QueryRow(ctx, q, args...).Scan(
		&s.ID, &s.ServiceName, &s.Price, &s.UserID, &s.StartDate, &s.EndDate,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Subscription{}, ErrNotFound
	}
	return s, err
}

func (r *SubscriptionsRepo) List(ctx context.Context, f ListFilter) ([]Subscription, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	where := []string{"1=1"}
	args := make([]any, 0, 4)
	n := 1

	if f.UserID != nil && strings.TrimSpace(*f.UserID) != "" {
		where = append(where, fmt.Sprintf("user_id=$%d", n))
		args = append(args, *f.UserID)
		n++
	}
	if f.ServiceName != nil && strings.TrimSpace(*f.ServiceName) != "" {
		where = append(where, fmt.Sprintf("service_name=$%d", n))
		args = append(args, *f.ServiceName)
		n++
	}

	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf(`
		SELECT id, service_name, price, user_id, start_date, end_date
		FROM subscriptions
		WHERE %s
		ORDER BY start_date DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(where, " AND "), n, n+1)

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.ID, &s.ServiceName, &s.Price, &s.UserID, &s.StartDate, &s.EndDate); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *SubscriptionsRepo) TotalCost(ctx context.Context, from, to time.Time, userID *string, serviceName *string) (int64, error) {
	where := []string{"1=1"}
	args := []any{from, to}
	n := 3

	if userID != nil && strings.TrimSpace(*userID) != "" {
		where = append(where, fmt.Sprintf("s.user_id=$%d", n))
		args = append(args, *userID)
		n++
	}
	if serviceName != nil && strings.TrimSpace(*serviceName) != "" {
		where = append(where, fmt.Sprintf("s.service_name=$%d", n))
		args = append(args, *serviceName)
		n++
	}

	q := fmt.Sprintf(`
		WITH gs AS (
			SELECT generate_series($1::date, $2::date, interval '1 month')::date AS m
		)
		SELECT COALESCE(SUM(s.price), 0)
		FROM gs
		JOIN subscriptions s
		  ON s.start_date <= gs.m
		 AND (s.end_date IS NULL OR s.end_date >= gs.m)
		WHERE %s
	`, strings.Join(where, " AND "))

	var total int64
	if err := r.db.Pool.QueryRow(ctx, q, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}
