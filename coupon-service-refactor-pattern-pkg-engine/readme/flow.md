coupon-service ของคุณทำ 3 บทบาทหลัก

🎯 Admin จัดการคูปอง (HTTP)

🧮 Order ขอคำนวณคูปอง (gRPC)

👤 User ดู / เก็บคูปอง (HTTP)

📣 Event-driven sync ด้วย Kafka

ทุกอย่างถูกแยกตาม Clean Architecture + CQRS + Event-driven
------------------------------------------------------------------------------------
1️⃣ FLOW: Admin สร้าง / แก้ / ปิดคูปอง
🔹 จุดเริ่ม
 Admin UI / Backoffice
- POST /admin/coupons
- PUT  /admin/coupons/{code}
- DELETE /admin/coupons/{code}
//--//--//
🔹 FLOW: Admin สร้าง / แก้ / ปิดคูปอง
Admin HTTP
↓
adapter/http/admin/coupon_handler.go
↓
service/admin/CouponManagementService
↓
repository (Postgres)
↓
Kafka Publisher
//--//--//
🔹 Create Coupon
Admin → HTTP
→ CreateCoupon()
→ CouponManagementService.CreateCoupon()
→ repo.Save()
→ publisher.Publish(CouponCreated)
→ Kafka
//--//--//
📌 ส่งตอนไหน
- หลัง DB save สำเร็จแล้ว (สำคัญมาก)
📌 คุมยังไง
- DB เป็น source of truth
- Kafka เป็น eventual consistency

##################################################################################

2️⃣ FLOW: User เปิดหน้าแอป → เห็นคูปอง
🔹 จุดเริ่ม

Frontend / Mobile App
- GET /coupons

🔹 ลำดับการทำงาน
HTTP
↓
adapter/http/public/coupon_handler.go
↓
CouponQueryService
↓
CouponRepository
↓
Postgres
//--//--//
📌 ส่งอะไรกลับ
  { "code": "NEW100", "discount": 100 }
📌 ไม่มี Kafka
เพราะเป็น pure read (CQRS)
#######################################################################
3️⃣ FLOW: User กด “เก็บคูปอง” (Claim)
🔹 จุดเริ่ม

Frontend

POST /coupons/{code}/claim

🔹 ลำดับการทำงาน (ที่ควรเป็น)
HTTP
↓
CouponHTTPHandler
↓
CouponClaimService   ⭐ (logic สำคัญ)
↓
repo.IncreaseUsage()  ← คุม quota
↓
repo.Save(user_coupon)
↓
publisher.Publish(CouponClaimed)
↓
Kafka
//--//--//
📌 คุม quota ยังไง
- UPDATE coupons
- SET used = used + 1
- WHERE used < max_usage
→ atomic / safe
//--//--//
📌 Kafka event
CouponClaimed {
  userID,
  couponCode
}
📌 ใครควรฟัง
- user-coupon-service
- vnotification-service
###########################################################################
4️⃣ FLOW: Order ใช้คูปอง (Apply / Redeem)
🔹 จุดเริ่ม
Order Service
gRPC ApplyCoupon
🔹 ลำดับการทำงาน
Order Service
↓ gRPC
adapter/grpc/order_coupon.go
↓
CouponCalculatorService
↓
CouponRepository
↓
Postgres
//--//--//
📌 ทำอะไร
- validate coupon
- คำนวณส่วนลด
❌ ไม่ตัด quota

❌ ไม่ mark used
📌 response
{
  final_total: 900,
  discount: 100,
  valid: true
}
🔹 แล้ว Redeem ตอนไหน?
หลัง order จ่ายเงินจริงสำเร็จ
Order Service
↓
CouponCommandService.RedeemCoupon()
↓
Kafka (CouponRedeemed)
####################################################################
5️⃣ FLOW: Kafka Consumer (หัวใจ Event-driven)
🔹 จุดเริ่ม
- Kafka Message
- CouponCreated / Updated / Redeemed

🔹 ลำดับ
Kafka
↓
adapter/kafka/coupon_event_consumer.go
↓
service/CouponCommandService (Handle)
↓
repo.Save / repo.Deactivate / IncreaseUsage

📌 ทำหน้าที่

- sync state

- update read model

- idempotent handling

############################################################
📌🔹 สรุป “ส่งอะไร ตอนไหน” 🔹
เหตุการณ์	          ใครส่ง	                    ส่งอะไร	                ส่งไปไหน
Admin   create	    CouponManagementService	    CouponCreated	        Kafka
User claim	        CouponClaimService	        CouponClaimed	        Kafka
Order apply	        gRPC	                    Calculate only	        sync
Order paid	        Order svc	                CouponRedeemed	        Kafka
Kafka event	        Consumer	                update DB	            internal
🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹🔹

หน้าที่ coupon-service

เชื่อมต่อ frontend ด้วย http (global)

ส่ง สร้าง ลบ อัพเดท ด้วย kafka ไป user-coupon-service

