CREATE TABLE IF NOT EXISTS issues (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    audit_id BIGINT NULL,
    title VARCHAR(220) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'open',
    assigned_to VARCHAR(150),
    due_date DATE NULL,
    corrective_action TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_issue_status(status),
    INDEX idx_issue_due_date(due_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
