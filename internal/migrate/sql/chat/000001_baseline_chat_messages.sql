CREATE TABLE IF NOT EXISTS chat_messages (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(64) NOT NULL,
    user_name VARCHAR(120) NOT NULL,
    role VARCHAR(30) NOT NULL,
    body VARCHAR(1000) NOT NULL,
    created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    INDEX idx_chat_created_at(created_at),
    INDEX idx_chat_user(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
