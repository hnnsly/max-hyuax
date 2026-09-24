-- +goose Up

-- Синтетический телефон демо-жительницы Анны: без клиента MAX номер не оставить, а УК в демо
-- должна видеть блок «Контакты жителей». Номер вымышленный, помечен в docs/data.md.
UPDATE users SET phone = '+79990000001' WHERE demo_key = 'resident_demo_1';

-- +goose Down
UPDATE users SET phone = '' WHERE demo_key = 'resident_demo_1';
