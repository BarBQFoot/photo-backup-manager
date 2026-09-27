// Run from the project root with: go run ./repository/main
package main

import (
	"log"

	"photo-backup-manager/repository"

	"gorm.io/gen"
	"gorm.io/gen/field"
)

func main() {
	// 1. ตั้งค่า Generator และตำแหน่งไฟล์ปลายทาง
	g := gen.NewGenerator(gen.Config{
		OutPath:      "model/query",
		ModelPkgPath: "model",
		Mode:         gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface,
	})

	// 2. ดึงการเชื่อมต่อกับ Database
	db, err := repository.NewDbConnection()
	if err != nil {
		log.Fatal(err)
	}
	g.UseDB(db)

	// แสดงรายชื่อตารางที่พบในฐานข้อมูล (มี sqlite_sequence เป็นตารางระบบได้)
	tables, err := db.Migrator().GetTables()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Tables: %v", tables)

	// กำหนด Models จากตารางในระบบ
	backupJob := g.GenerateModel("backup_jobs")

	// file_records มี Foreign Key backup_job_id ไปยัง backup_jobs.id
	fileRecordBelongsToJob := gen.FieldRelate(
		field.BelongsTo,
		"BackupJob",
		backupJob,
		&field.RelateConfig{
			GORMTag: field.GormTag{
				"foreignKey": []string{"BackupJobID"},
				"references": []string{"ID"},
			},
		},
	)
	fileRecord := g.GenerateModel("file_records", fileRecordBelongsToJob)

	// 3. ระบุตารางที่ต้องการสร้าง Model และ Query
	g.ApplyBasic(backupJob, fileRecord)

	// 4. สั่งสร้างโค้ดอัตโนมัติ
	g.Execute()
}
