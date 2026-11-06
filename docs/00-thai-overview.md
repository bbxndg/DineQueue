# DineQueue — คู่มือภาพรวมโปรเจกต์ภาษาไทย

> แหล่งอ้างอิง: `group_work_specs_v1.pdf` ส่วน 1–12 โดยเฉพาะ Topic 9 หน้า 8–9, Sprint Model หน้า 9–10, Engineering Artifacts หน้า 10–11 และ Rubric หน้า 11–15
> เอกสารในชุดนี้เป็น **ข้อเสนอออกแบบ** ไม่ใช่ระบบที่เขียนโค้ดแล้ว และไม่ได้แปลว่าอาจารย์อนุมัติทุก Design Decision

## โปรเจกต์ทำอะไร

DineQueue เป็นระบบคิวร้านอาหาร Walk-in ลูกค้าเข้าระบบ ระบุจำนวนคน รับเลขคิว และติดตามสถานะ พนักงานร้านอาหารเปิด Dashboard เรียกคิวถัดไปและปิดคิว เมื่อขยายใน Sprint 2 สามารถตรวจสถานะโต๊ะและจัดกลุ่มลูกค้าให้โต๊ะที่มีจำนวนที่นั่งเพียงพอได้ โดยต้องป้องกันการจัดโต๊ะซ้ำเวลาพนักงานหลายคนกดพร้อมกัน

### เป้าหมายและข้อจำกัด

- เริ่มร้านอาหาร 1 สาขา คิวบริการ 1 คิวก่อนใน Sprint 1
- User Roles: CUSTOMER และ HOST เท่านั้นใน Sprint 1; 2 roles เพียงพอ
- ไม่ต้องทำระบบสมัครสมาชิก ลืมรหัสผ่าน OTP หรือการส่ง SMS ใน Sprint 1
- ไม่มีจองโต๊ะล่วงหน้า อาหาร POS หรือชำระเงินจริง
- ต้องมี UI + API + persistent DB จริง ต้องเก็บข้อมูลหลังรีสตาร์ต
- Database constraints ช่วยรักษากฎภายใต้ concurrent requests

## Sprint 1: ขั้นต่ำที่ต้องทำให้เสร็จจริงก่อน 8 พ.ย.

1. **Auth:** Login, Logout, Current User, password hashing, session lifecycle, RBAC ที่ตรวจบน Backend
2. **Customer:** เข้าคิวโดยระบุ Party Size, ได้เลข A-001/A-002, ดูคิวตัวเองอย่างเป็นส่วนตัว
3. **Host:** Login, ดูคิวที่ได้รับอนุญาต, Call Next ตาม FIFO, Complete คิวที่ถูกเรียก
4. **Business Rules:** ห้ามคิว Active ซ้ำ, ห้ามเลขคิวซ้ำ, ห้ามเรียกผิดลำดับ/เปลี่ยนสถานะผิด, ห้ามคนไม่มีสิทธิ์
5. **Tests:** มี Unit, API+Database Integration, E2E สำหรับ Critical Journey, Negative, Boundary, Concurrency และหลักฐาน TDD ที่รันจริง
6. **Process:** ใช้ Issue, Feature Branch, PR, Review เพื่อนร่วมทีม, Sprint Staging, Integration Test, Main Release Tag, Setup/Seed/Demo Instructions
7. **Documentation:** SRS, Feature Inventory, Traceability, System-Level SDS, Three-Sprint Plan, Feature-Level Contracts, AC, STS, DoD และ Evidence Index

**ห้ามสรุปว่างานเสร็จเพียงเพราะมีเอกสาร:** Deliverable 1 ยังต้องมีแอปที่ทำงานจริง และหลักฐานทดสอบ/PR/Review ตาม Git SHA ที่ส่ง

## Sprint 2: ภายใน 29 พ.ย.

- ตารางโต๊ะ: รหัสโต๊ะ ขนาดที่นั่ง สถานะ AVAILABLE/OCCUPIED
- Assign Table: ต้องโต๊ะว่าง รองรับจำนวนคน และไม่ถูกจัดซ้ำ
- การเรียกกลุ่มที่โต๊ะรองรับได้: เลือก FIFO เฉพาะกลุ่มที่เหมาะสมภายใต้นโยบายที่ยืนยันกับ TA
- ยกเลิกคิว WAITING, ทำ No-show จาก CALLED, ปลดล็อกโต๊ะหลังใช้งาน
- Feature Contracts และ Tests ต้องเขียนก่อน implement; Sprint 2 ทุกฟีเจอร์ต้องเสร็จและปล่อย integrated release

## Sprint 3: เสร็จ 13 ธ.ค.

- Queue History, จำนวนคิว/เวลารอจริง, dashboard summary
- Public Queue Display แบบไม่เปิดข้อมูลลูกค้า (ถ้าเลือกเก็บใน scope)
- แก้ UAT defects, จัดการ failure paths, สิทธิ์, usability, regression tests
- Final verified product + PDF evidence report + deployed URL + วิดีโอไม่เกิน 10 นาที ทั้ง 2 คนมีส่วนร่วม

## Lab ที่ต้องทำควบคู่กัน

- 15 พ.ย. **Lab 5:** Docker, CI/CD quality gate, Deploy GCP, พิสูจน์ Database Persistence
- 22 พ.ย. **Lab 6:** แลก UAT กับอีกทีม 4 กรณี, defect issue, fix และให้ทีมอื่น retest
- 29 พ.ย. **Lab 7:** ทำ change ที่ TA/อาจารย์อนุมัติ, impact analysis, failing test ก่อนแก้, regression, metrics 5 ค่า และ readiness

## กฎสำคัญที่ทีมต้องรู้

- **BR-001:** คนเดียวกันมี Active Ticket ในคิวบริการเดียวกันได้เพียงหนึ่งใบ
- **BR-002:** เลขคิวต้องไม่ซ้ำภายใน queue + local date แม้มีคน Join พร้อมกัน
- **BR-003:** Sprint 1 เรียกคิวเรียงเวลารับคิวแบบ FIFO
- **BR-004:** Sprint 1 `WAITING -> CALLED -> COMPLETED` เท่านั้น
- **BR-005:** เรียกคิวใหม่ไม่ได้ถ้ามีคิว CALLED ค้างอยู่
- **BR-006/007:** Host ต้องมีสิทธิ์ในคิว และ Customer เห็นได้เฉพาะคิวของตัวเอง
- **BR-010 (Sprint 2):** โต๊ะต้องว่างและมี capacity >= party size; assignment ใช้ transaction
- **BR-011 (Sprint 2):** First Eligible Table Matching ต้องนิยามเรื่องการข้ามคิวให้ชัดกับ TA

## โครงสร้างที่ต้องส่งไปให้ Codex

เริ่มจาก `AGENTS.md`, `docs/01-srs.md`, `docs/03-system-sds.md`, `docs/04-data-and-api.md`, `docs/features/sprint1-contracts.md`, `docs/testing/sprint1-sts.md` และ `docs/09-traceability.md`. ใช้เอกสารเหล่านี้เป็น source of truth ระหว่างสร้างโค้ด แต่ต้อง verify ความถูกต้องและผลทดสอบด้วยมนุษย์ทั้งสองคน

## สิ่งที่ต้องขอคำตอบ/ยืนยันก่อนเริ่มหรือก่อน Sprint 2

1. ทีมมีสิทธิ์เลือก Topic 9 ตามเลขหลักสุดท้ายของ Student ID หรือไม่
2. ในร้านอาหาร `COMPLETED` หมายถึงเรียกและจัดให้ลูกค้าได้โต๊ะแล้วใช่หรือไม่
3. โต๊ะว่าง 2 ที่นั่ง สามารถเรียกกลุ่ม 2 คนข้ามกลุ่ม 6 คนที่มาก่อนหรือไม่
4. วันใหม่ของเลขคิวเริ่มกี่โมง และขนาดกลุ่มสูงสุดเท่าไร
5. ใช้ GCP resources อะไรตาม Lab5 handout และต้องมีหลักฐานอะไรเพิ่มเติม

รายละเอียดเต็ม รวม UI/API/Database, AC, STS, Traceability, Process และ Submission อยู่ในไฟล์ภาษาอังกฤษที่เหลือใน `docs/`


## อัปเดต v2: Restaurant Operations + รวมโต๊ะ

- **ลูกค้ากลุ่ม 10 คนรับคิวได้** (`party_size` 1–16) แม้โต๊ะเดี่ยวรองรับได้สูงสุด 8 คน; Sprint 1 ยังไม่ต้องจัดโต๊ะจริง
- Sprint 2 จัดการ Floor/Zone/Table State, ตารางการเชื่อมโต๊ะที่อนุมัติ, Smart Matching แบบอธิบายเหตุผลได้, Call + Hold, Seat, Checkout/Cleaning และ Concurrent Allocation
- **ตัวอย่าง:** T4 (6 ที่) ติดกับ T5 (4 ที่) → รวมได้ 10 ที่ จึงรองรับกลุ่ม 10 คน; T4 + โต๊ะที่อยู่อีกโซนหรือไม่เชื่อมกันห้ามรวม
- Sprint 3 แสดง Analytics ที่วัดจริง, Queue Tracking/ETA อย่างระมัดระวัง, History, การตั้งค่าผังร้านและ Testing/Release Hardening
- ข้อกำหนดนี้เป็น Design Decisions ของทีมที่ต้องตกลงและยืนยันกับ TA; เอกสารอาจารย์บังคับขอบเขตและกระบวนการ Engineering ไม่ได้บังคับ Algorithm หรือจำนวนโต๊ะ
- สเปกละเอียดอยู่ที่ `docs/06-restaurant-operations.md` และ `docs/features/sprint2-restaurant-operations-contracts.md`
