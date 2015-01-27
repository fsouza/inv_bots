CREATE TABLE cvm_material_facts (
	fact_id SERIAL PRIMARY KEY,
	send_date VARCHAR(55) NOT NULL,
	reference_date VARCHAR(55) NOT NULL,
	company VARCHAR(255) NOT NULL,
	subject VARCHAR(255) NOT NULL,
	protocol_number VARCHAR(255) NOT NULL
);
