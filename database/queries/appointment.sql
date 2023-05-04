
-- name: CreateAppointment :one
WITH schedule AS (
	WITH weekdays AS (
		SELECT (enum_range(NULL::weekdays))[EXTRACT(dow FROM @starting)] AS weekday
	)
	SELECT starting_time, ending_time FROM schedules WHERE weekday = weekdays.weekday AND schedules.account_id = @personnel_id
), service AS (
	SELECT duration FROM services WHERE service_id = @service_id
)
INSERT INTO appointments(service_id, account_id, starting)
	SELECT @service_id, @account_id, @starting
		WHERE starting >= schedule.starting_time AND (starting + service.duration) <= schedule.ending_time
		RETURNING appointment_id;



-- name: GetTimetableForDate :many
WITH weekdays AS (
		SELECT (enum_range(NULL::weekdays))[EXTRACT(dow FROM @desired_date)] AS weekday
), schedule AS (
	-- get the working hours for the member
	SELECT starting_time, ending_time FROM schedules, weekdays WHERE schedules.weekday = weekdays.weekday AND schedules.account_id = @personnel_id
), idx AS (
	-- get the appointments for that member
	SELECT 
		-- get the index in their working hours
		(extract(epoch FROM (starting - (date_trunc('day', starting)+schedule.starting_time)))/(60 * 30)) AS idx,
		-- convert the duration to a number of indices that should be blocked
		(extract(epoch FROM (duration))/(60 * 30)) AS delta
	FROM appointments, schedule JOIN services ON service_id = service_id
	-- where date of starting = date of desired date, ignoring the time component
	WHERE date_trunc('day', appointments.starting) = date_trunc('day', @desired_date) 
), series AS (
	-- generate a list of indices for the member's working hours, for the timetable
	SELECT generate_series(0, (extract(epoch FROM (schedule.ending_time - schedule.starting_time))/(60 * 30)) - 1, 1) AS indices FROM schedule
), indices AS (
	-- generate a list of blocked entries in the timetable
	SELECT generate_series(idx, idx + delta -1, 1 ) AS block_index FROM idx ORDER BY block_index
)
SELECT series.indices, schedule.starting_time+(series.indices*'30m'::interval) AS times , (block_index IS NOT NULL) as is_blocked FROM schedule, series LEFT JOIN indices ON series.indices = indices.block_index ORDER BY series.indices;

