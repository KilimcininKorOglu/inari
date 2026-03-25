CREATE FUNCTION get_user_payments(uid INTEGER)
RETURNS TABLE(amount DECIMAL, status VARCHAR)
AS $$
BEGIN
  RETURN QUERY SELECT p.amount, p.status FROM payments p WHERE p.user_id = uid;
END;
$$ LANGUAGE plpgsql;
