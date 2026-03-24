-- 车辆模拟数据 (使用 REPLACE 避免 ID 重复报错)
REPLACE INTO `vehicle` (`id`, `lon`, `lat`, `speed`, `update_time`, `status`, `tybe`, `size`, `capacity`) VALUES
(1, 116.40, 39.90, 60.0, NOW(), 2, 1, 10, 1000), -- 空闲 普通
(2, 116.41, 39.91, 55.0, NOW(), 1, 1, 10, 800),  -- 运行中 普通
(3, 116.42, 39.92, 0.0,  NOW(), 2, 2, 8,  500),  -- 空闲 危化品
(4, 116.43, 39.93, 40.0, NOW(), 1, 1, 15, 2000), -- 运行中 大容量
(5, 116.44, 39.94, 0.0,  NOW(), 2, 1, 5,  300),  -- 空闲 小车
(6, 116.45, 39.95, 0.0,  NOW(), 2, 1, 10, 1000); -- 空闲 普通

-- 货物类型模拟数据
REPLACE INTO `cargo` (`id`, `name`, `tybe`, `pack`, `weight`) VALUES
(1, 1, 1, 1, 200),  -- 普通包裹 200kg
(2, 2, 2, 2, 400),  -- 液体化学品 (危险品) 400kg
(3, 3, 1, 1, 1500); -- 大型机械 1500kg

-- 订单/运单模拟数据
REPLACE INTO `shipment` (`id`, `start_poi_id`, `end_poi_id`, `create_time`, `update_time`, `status`, `cargo_id`, `count`) VALUES
(1, 1, 2, DATE_SUB(NOW(), INTERVAL 1 HOUR), NOW(), 4, 1, 1), -- 已完成
(2, 2, 3, DATE_SUB(NOW(), INTERVAL 2 HOUR), NOW(), 3, 2, 1), -- 运输中
(3, 1, 3, NOW(), NOW(), 1, 1, 1);                            -- 待分配

-- 任务模拟数据
REPLACE INTO `order_task` (`id`, `shipment_id`, `vehicle_id`, `sequential`, `create_time`, `update_time`) VALUES
(1, 1, 1, 3, DATE_SUB(NOW(), INTERVAL 1 HOUR), DATE_SUB(NOW(), INTERVAL 20 MINUTE)), -- 已完成任务
(2, 2, 2, 2, DATE_SUB(NOW(), INTERVAL 2 HOUR), DATE_SUB(NOW(), INTERVAL 90 MINUTE)); -- 运输中任务

