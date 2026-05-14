package services

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ablate-ai/RuleFlow/database"
)

const backupRetentionCount = 6

// backupKeyPattern 合法备份文件名格式：ruleflow/backup-2006-01-02-15-04-05.tar.gz
var backupKeyPattern = regexp.MustCompile(`^ruleflow/backup-\d{4}-\d{2}-\d{2}-\d{2}-\d{2}-\d{2}\.tar\.gz$`)

// backupTables 备份时的导出顺序
var backupTables = []string{
	"subscriptions",
	"nodes",
	"templates",
	"rule_sources",
	"config_policies",
	"config_access_logs",
}

// restoreOrder 恢复时的 COPY 顺序（需先满足 FK：config_access_logs 依赖 config_policies）
var restoreOrder = []string{
	"subscriptions",
	"nodes",
	"templates",
	"rule_sources",
	"config_policies",
	"config_access_logs",
}

// R2Object R2 上的备份文件元数据
type R2Object struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

// BackupService 数据库备份服务
type BackupService struct {
	repo *database.BackupRepo
	pool *pgxpool.Pool
}

func NewBackupService(repo *database.BackupRepo, pool *pgxpool.Pool) *BackupService {
	return &BackupService{repo: repo, pool: pool}
}

func (s *BackupService) GetSettings(ctx context.Context) (*database.BackupSettings, error) {
	return s.repo.GetSettings(ctx)
}

func (s *BackupService) SaveSettings(ctx context.Context, settings *database.BackupSettings) error {
	return s.repo.SaveSettings(ctx, settings)
}


func (s *BackupService) newS3Client(settings *database.BackupSettings) *s3.Client {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", settings.R2AccountID)
	return s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true,
		Credentials: credentials.NewStaticCredentialsProvider(
			settings.R2AccessKeyID,
			settings.R2SecretAccessKey,
			"",
		),
	})
}

// TestConnection 用给定配置测试 R2 连通性
func (s *BackupService) TestConnection(ctx context.Context, settings *database.BackupSettings) error {
	client := s.newS3Client(settings)
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(settings.R2BucketName),
	})
	return err
}

// RunBackup 执行一次完整备份：导出所有表 CSV → tar.gz → 上传 R2 → 清理旧备份
func (s *BackupService) RunBackup(ctx context.Context) error {
	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		return fmt.Errorf("读取备份配置失败: %w", err)
	}
	if !settings.Enabled {
		return nil
	}

	fileKey := fmt.Sprintf("ruleflow/backup-%s.tar.gz", time.Now().UTC().Format("2006-01-02-15-04-05"))

	data, err := s.buildArchive(ctx)
	if err != nil {
		_ = s.repo.CreateRecord(ctx, &database.BackupRecord{
			FileKey:      fileKey,
			Status:       "failed",
			ErrorMessage: err.Error(),
		})
		return fmt.Errorf("构建备份包失败: %w", err)
	}

	client := s.newS3Client(settings)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(settings.R2BucketName),
		Key:           aws.String(fileKey),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   aws.String("application/gzip"),
	})
	if err != nil {
		_ = s.repo.CreateRecord(ctx, &database.BackupRecord{
			FileKey:      fileKey,
			FileSize:     int64(len(data)),
			Status:       "failed",
			ErrorMessage: err.Error(),
		})
		return fmt.Errorf("上传 R2 失败: %w", err)
	}

	rec := &database.BackupRecord{
		FileKey:  fileKey,
		FileSize: int64(len(data)),
		Status:   "success",
	}
	if err := s.repo.CreateRecord(ctx, rec); err != nil {
		log.Printf("[backup] 写入备份记录失败: %v", err)
	}

	// 清理超出保留数量的旧备份
	deleted, err := s.repo.PruneOldRecords(ctx, backupRetentionCount)
	if err != nil {
		log.Printf("[backup] 清理旧记录失败: %v", err)
	}
	for _, d := range deleted {
		if d.FileKey == "" {
			continue
		}
		if _, delErr := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(settings.R2BucketName),
			Key:    aws.String(d.FileKey),
		}); delErr != nil {
			log.Printf("[backup] 删除 R2 旧文件失败 %s: %v", d.FileKey, delErr)
		}
	}

	log.Printf("[backup] 备份完成: %s (%.1f KB)", fileKey, float64(len(data))/1024)
	return nil
}

// buildArchive 将所有表导出为 CSV，打包成 tar.gz 返回字节
func (s *BackupService) buildArchive(ctx context.Context) ([]byte, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for _, table := range backupTables {
		csvData, err := s.tableToCSV(ctx, table)
		if err != nil {
			return nil, fmt.Errorf("导出表 %s 失败: %w", table, err)
		}
		hdr := &tar.Header{
			Name:    table + ".csv",
			Size:    int64(len(csvData)),
			Mode:    0644,
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(csvData); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// tableToCSV 查询整张表，返回 CSV 字节（含表头）
func (s *BackupService) tableToCSV(ctx context.Context, table string) ([]byte, error) {
	// 表名已在 backupTables 白名单中，无 SQL 注入风险
	rows, err := s.pool.Query(ctx, "SELECT * FROM "+table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// 写表头
	descs := rows.FieldDescriptions()
	headers := make([]string, len(descs))
	for i, d := range descs {
		headers[i] = string(d.Name)
	}
	if err := w.Write(headers); err != nil {
		return nil, err
	}

	// 写数据行
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		record := make([]string, len(vals))
		for i, v := range vals {
			if v == nil {
				record[i] = ""
			} else {
				record[i] = strings.TrimRight(fmt.Sprintf("%v", v), " ")
			}
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// ListR2Objects 列出 R2 bucket 中的所有备份文件
func (s *BackupService) ListR2Objects(ctx context.Context) ([]*R2Object, error) {
	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取备份配置失败: %w", err)
	}
	client := s.newS3Client(settings)
	out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(settings.R2BucketName),
		Prefix: aws.String("ruleflow/"),
	})
	if err != nil {
		return nil, fmt.Errorf("列出 R2 文件失败: %w", err)
	}
	var objects []*R2Object
	for _, obj := range out.Contents {
		o := &R2Object{Key: aws.ToString(obj.Key), Size: aws.ToInt64(obj.Size)}
		if obj.LastModified != nil {
			o.LastModified = *obj.LastModified
		}
		objects = append(objects, o)
	}
	// 按时间倒序
	for i, j := 0, len(objects)-1; i < j; i, j = i+1, j-1 {
		objects[i], objects[j] = objects[j], objects[i]
	}
	return objects, nil
}

// RestoreFromKey 从 R2 下载指定备份文件并恢复到数据库（事务执行，失败回滚）
func (s *BackupService) RestoreFromKey(ctx context.Context, fileKey string) error {
	if !backupKeyPattern.MatchString(fileKey) {
		return fmt.Errorf("无效的备份文件名: %q", fileKey)
	}

	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		return fmt.Errorf("读取备份配置失败: %w", err)
	}

	// 从 R2 下载
	client := s.newS3Client(settings)
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(settings.R2BucketName),
		Key:    aws.String(fileKey),
	})
	if err != nil {
		return fmt.Errorf("从 R2 下载失败: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取下载内容失败: %w", err)
	}

	// 解压 tar.gz，提取各表 CSV
	csvMap, err := extractArchive(data)
	if err != nil {
		return fmt.Errorf("解压备份文件失败: %w", err)
	}

	// 用单条连接执行事务，方便 COPY FROM STDIN
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}
	defer conn.Release()

	pgConn := conn.Conn().PgConn()

	if err := pgExec(ctx, pgConn, "BEGIN"); err != nil {
		return err
	}

	// TRUNCATE 所有应用表（CASCADE 处理 FK）
	truncateSQL := `TRUNCATE subscriptions, nodes, templates, rule_sources, config_policies, config_access_logs RESTART IDENTITY CASCADE`
	if err := pgExec(ctx, pgConn, truncateSQL); err != nil {
		_ = pgExec(ctx, pgConn, "ROLLBACK")
		return fmt.Errorf("清空表失败: %w", err)
	}

	// 按恢复顺序逐表 COPY FROM STDIN
	for _, table := range restoreOrder {
		csvData, ok := csvMap[table+".csv"]
		if !ok || len(csvData) == 0 {
			continue
		}
		copySQL := fmt.Sprintf("COPY %s FROM STDIN WITH (FORMAT CSV, HEADER true)", table)
		if _, err := pgConn.CopyFrom(ctx, bytes.NewReader(csvData), copySQL); err != nil {
			_ = pgExec(ctx, pgConn, "ROLLBACK")
			return fmt.Errorf("恢复表 %s 失败: %w", table, err)
		}
	}

	if err := pgExec(ctx, pgConn, "COMMIT"); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	log.Printf("[backup] 从 %s 恢复完成", fileKey)
	return nil
}

// extractArchive 解压 tar.gz，返回 filename→content 映射
func extractArchive(data []byte) (map[string][]byte, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	result := make(map[string][]byte)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		result[hdr.Name] = content
	}
	return result, nil
}

// pgExec 在 pgconn 上执行简单 SQL 并丢弃结果
func pgExec(ctx context.Context, conn *pgconn.PgConn, sql string) error {
	return conn.Exec(ctx, sql).Close()
}
