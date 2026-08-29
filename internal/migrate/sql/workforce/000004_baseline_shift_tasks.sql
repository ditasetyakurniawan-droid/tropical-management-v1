CREATE TABLE IF NOT EXISTS shift_tasks (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    shift_date DATE NOT NULL,
    title VARCHAR(180) NOT NULL,
    station VARCHAR(40) NOT NULL,
    assigned_to_id BIGINT NOT NULL DEFAULT 0,
    assigned_to_name VARCHAR(120) NOT NULL DEFAULT 'Semua Tim',
    priority VARCHAR(16) NOT NULL DEFAULT 'normal',
    status VARCHAR(24) NOT NULL DEFAULT 'open',
    created_by_id BIGINT NOT NULL,
    created_by_name VARCHAR(120) NOT NULL,
    completed_by_id BIGINT NULL,
    completed_by_name VARCHAR(120) NOT NULL DEFAULT '',
    completed_at DATETIME NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_tasks_date_status (shift_date, status),
    INDEX idx_tasks_assignee_date (assigned_to_id, shift_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
