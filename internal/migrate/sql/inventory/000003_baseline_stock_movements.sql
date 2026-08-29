CREATE TABLE IF NOT EXISTS stock_movements (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    item_id BIGINT NOT NULL,
    delta DECIMAL(12,2) NOT NULL,
    reason VARCHAR(220) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_movement_item(item_id),
    INDEX idx_movement_created(created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
