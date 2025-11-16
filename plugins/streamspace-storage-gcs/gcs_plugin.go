package main

import ("context"; "encoding/json"; "fmt"; "github.com/yourusername/streamspace/api/internal/plugins"; "cloud.google.com/go/storage"; "google.golang.org/api/option")

type GCSPlugin struct {
	plugins.BasePlugin
	config GCSConfig
	client *storage.Client
	bucket *storage.BucketHandle
}

type GCSConfig struct {
	Enabled         bool          `json:"enabled"`
	ProjectID       string        `json:"projectID"`
	BucketName      string        `json:"bucketName"`
	CredentialsJSON string        `json:"credentialsJSON"`
	StoragePaths    StoragePaths  `json:"storagePaths"`
}

type StoragePaths struct {
	Recordings string `json:"recordings"`
	Snapshots  string `json:"snapshots"`
	Uploads    string `json:"uploads"`
}

func (p *GCSPlugin) Initialize(ctx *plugins.PluginContext) error {
	configBytes, _ := json.Marshal(ctx.Config)
	json.Unmarshal(configBytes, &p.config)

	if !p.config.Enabled {
		ctx.Logger.Info("Google Cloud Storage is disabled")
		return nil
	}

	// Create GCS client with service account credentials
	client, err := storage.NewClient(
		context.Background(),
		option.WithCredentialsJSON([]byte(p.config.CredentialsJSON)),
	)
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %w", err)
	}

	p.client = client
	p.bucket = client.Bucket(p.config.BucketName)

	// Verify bucket access
	_, err = p.bucket.Attrs(context.Background())
	if err != nil {
		ctx.Logger.Warn("Failed to access GCS bucket (will retry later)", "bucket", p.config.BucketName, "error", err)
	}

	ctx.Logger.Info("Google Cloud Storage initialized", "project", p.config.ProjectID, "bucket", p.config.BucketName)
	return nil
}

func (p *GCSPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Google Cloud Storage plugin loaded")
	return nil
}

// UploadFile uploads a file to GCS
func (p *GCSPlugin) UploadFile(path string, data []byte) error {
	obj := p.bucket.Object(path)
	w := obj.NewWriter(context.Background())
	defer w.Close()

	_, err := w.Write(data)
	return err
}

// DownloadFile downloads a file from GCS
func (p *GCSPlugin) DownloadFile(path string) ([]byte, error) {
	obj := p.bucket.Object(path)
	r, err := obj.NewReader(context.Background())
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return ioutil.ReadAll(r)
}

// DeleteFile deletes a file from GCS
func (p *GCSPlugin) DeleteFile(path string) error {
	obj := p.bucket.Object(path)
	return obj.Delete(context.Background())
}

// ListFiles lists files in a path
func (p *GCSPlugin) ListFiles(prefix string) ([]string, error) {
	it := p.bucket.Objects(context.Background(), &storage.Query{
		Prefix: prefix,
	})

	files := []string{}
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		files = append(files, attrs.Name)
	}
	return files, nil
}

func init() {
	plugins.Register("streamspace-storage-gcs", &GCSPlugin{})
}
