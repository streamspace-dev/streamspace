package main

import ("encoding/json"; "fmt"; "github.com/yourusername/streamspace/api/internal/plugins"; "github.com/aws/aws-sdk-go/aws"; "github.com/aws/aws-sdk-go/aws/credentials"; "github.com/aws/aws-sdk-go/aws/session"; "github.com/aws/aws-sdk-go/service/s3")

type S3Plugin struct {
	plugins.BasePlugin
	config S3Config
	client *s3.S3
}

type S3Config struct {
	Enabled         bool          `json:"enabled"`
	Provider        string        `json:"provider"`
	Endpoint        string        `json:"endpoint"`
	Region          string        `json:"region"`
	Bucket          string        `json:"bucket"`
	AccessKeyID     string        `json:"accessKeyID"`
	SecretAccessKey string        `json:"secretAccessKey"`
	UseSSL          bool          `json:"useSSL"`
	PathStyle       bool          `json:"pathStyle"`
	StoragePaths    StoragePaths  `json:"storagePaths"`
	Encryption      Encryption    `json:"encryption"`
}

type StoragePaths struct {
	Recordings string `json:"recordings"`
	Snapshots  string `json:"snapshots"`
	Uploads    string `json:"uploads"`
}

type Encryption struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"`
	KMSKeyID  string `json:"kmsKeyID"`
}

func (p *S3Plugin) Initialize(ctx *plugins.PluginContext) error {
	configBytes, _ := json.Marshal(ctx.Config)
	json.Unmarshal(configBytes, &p.config)

	if !p.config.Enabled {
		ctx.Logger.Info("S3 storage is disabled")
		return nil
	}

	// Create AWS session
	awsConfig := &aws.Config{
		Region:      aws.String(p.config.Region),
		Credentials: credentials.NewStaticCredentials(p.config.AccessKeyID, p.config.SecretAccessKey, ""),
	}

	if p.config.Endpoint != "" {
		awsConfig.Endpoint = aws.String(p.config.Endpoint)
		awsConfig.S3ForcePathStyle = aws.Bool(p.config.PathStyle)
	}

	if !p.config.UseSSL {
		awsConfig.DisableSSL = aws.Bool(true)
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %w", err)
	}

	p.client = s3.New(sess)

	// Verify bucket access
	_, err = p.client.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(p.config.Bucket),
	})
	if err != nil {
		ctx.Logger.Warn("Failed to access S3 bucket (will retry later)", "bucket", p.config.Bucket, "error", err)
	}

	ctx.Logger.Info("S3 storage initialized", "provider", p.config.Provider, "bucket", p.config.Bucket, "region", p.config.Region)
	return nil
}

func (p *S3Plugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("S3 Storage plugin loaded")
	return nil
}

// UploadFile uploads a file to S3
func (p *S3Plugin) UploadFile(path string, data []byte, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(p.config.Bucket),
		Key:         aws.String(path),
		Body:        aws.ReadSeekCloser(bytes.NewReader(data)),
		ContentType: aws.String(contentType),
	}

	if p.config.Encryption.Enabled {
		input.ServerSideEncryption = aws.String(p.config.Encryption.Algorithm)
		if p.config.Encryption.Algorithm == "aws:kms" && p.config.Encryption.KMSKeyID != "" {
			input.SSEKMSKeyId = aws.String(p.config.Encryption.KMSKeyID)
		}
	}

	_, err := p.client.PutObject(input)
	return err
}

// DownloadFile downloads a file from S3
func (p *S3Plugin) DownloadFile(path string) ([]byte, error) {
	result, err := p.client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(p.config.Bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()

	return ioutil.ReadAll(result.Body)
}

// DeleteFile deletes a file from S3
func (p *S3Plugin) DeleteFile(path string) error {
	_, err := p.client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(p.config.Bucket),
		Key:    aws.String(path),
	})
	return err
}

// ListFiles lists files in a path
func (p *S3Plugin) ListFiles(prefix string) ([]string, error) {
	result, err := p.client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(p.config.Bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, err
	}

	files := make([]string, len(result.Contents))
	for i, obj := range result.Contents {
		files[i] = *obj.Key
	}
	return files, nil
}

func init() {
	plugins.Register("streamspace-storage-s3", &S3Plugin{})
}
