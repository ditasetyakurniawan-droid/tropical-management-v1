-- Local development bootstrap only. Production databases live on DB-dt (192.168.100.70).
CREATE DATABASE IF NOT EXISTS tropical_auth CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS tropical_audit CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS tropical_inventory CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS tropical_sales CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS tropical_chat CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS tropical_workforce CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON tropical_auth.* TO 'tropical'@'%';
GRANT ALL PRIVILEGES ON tropical_audit.* TO 'tropical'@'%';
GRANT ALL PRIVILEGES ON tropical_inventory.* TO 'tropical'@'%';
GRANT ALL PRIVILEGES ON tropical_sales.* TO 'tropical'@'%';
GRANT ALL PRIVILEGES ON tropical_chat.* TO 'tropical'@'%';
GRANT ALL PRIVILEGES ON tropical_workforce.* TO 'tropical'@'%';
FLUSH PRIVILEGES;
