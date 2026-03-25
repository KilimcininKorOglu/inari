CREATE VIEW payment_summary AS
SELECT u.name, SUM(p.amount) as total
FROM users u
JOIN payments p ON u.id = p.user_id
GROUP BY u.name;
