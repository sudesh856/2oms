WITH original_company AS (
    SELECT id FROM companies WHERE name = 'Default Company' LIMIT 1
), ncm_locations AS (
    SELECT DISTINCT cl.location_name
    FROM courier_locations cl
    JOIN couriers c ON c.id = cl.courier_id AND c.company_id = cl.company_id
    CROSS JOIN original_company
    WHERE cl.company_id = original_company.id AND c.name = 'NCM'
)
DELETE FROM courier_locations cl
USING couriers c, original_company
WHERE cl.courier_id = c.id
    AND cl.company_id = c.company_id
    AND cl.company_id = original_company.id
    AND c.name IN ('Upaya/Delivery Sansar', 'Pathao/Doorma')
    AND cl.location_name IN (SELECT location_name FROM ncm_locations);