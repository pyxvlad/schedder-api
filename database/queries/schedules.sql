
-- name: SetSchedule :exec
INSERT INTO schedules (account_id, weekday, starting_time, ending_time) VALUES (@account_id, @weekday, @starting_time, @ending_time)
ON CONFLICT(account_id, weekday) DO
UPDATE SET starting_time = @starting_time, ending_time = @ending_time WHERE schedules.account_id = @account_id AND schedules.weekday = @weekday;

-- name: GetScheduleForWeekday :one
SELECT starting_time, ending_time FROM schedules WHERE account_id = @account_id AND weekday = @weekday;

-- name: GetSchedule :many
SELECT weekday, starting_time, ending_time FROM schedules WHERE account_id = @account_id;
