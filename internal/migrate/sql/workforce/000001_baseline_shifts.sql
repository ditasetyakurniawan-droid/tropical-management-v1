CREATE TABLE IF NOT EXISTS shifts (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    employee_id BIGINT NOT NULL,
    employee_name VARCHAR(120) NOT NULL,
    shift_date DATE NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    station VARCHAR(40) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'scheduled',
    notes VARCHAR(500) NOT NULL DEFAULT '',
    created_by_id BIGINT NOT NULL,
    created_by_name VARCHAR(120) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_shift_employee_start (employee_id, shift_date, start_time),
    INDEX idx_shifts_date_station (shift_date, station),
    INDEX idx_shifts_employee_date (employee_id, shift_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
