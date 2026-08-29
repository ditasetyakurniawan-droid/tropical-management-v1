CREATE TABLE IF NOT EXISTS sales_entries (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    business_date DATE NOT NULL,
    orders INT NOT NULL,
    revenue DECIMAL(14,2) NOT NULL,
    channel VARCHAR(60) NOT NULL DEFAULT 'dine-in',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_business_date(business_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
