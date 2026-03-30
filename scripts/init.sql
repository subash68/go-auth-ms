-- Initialisation script – runs once when the MySQL container is first created.
-- The database itself is created by the MYSQL_DATABASE env var; this script
-- only needs to create the table(s).

USE authdb;

CREATE TABLE IF NOT EXISTS Auth (
    ID          BIGINT       NOT NULL AUTO_INCREMENT,
    Token       VARCHAR(512) NOT NULL,
    Description TEXT,
    PRIMARY KEY (ID)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
