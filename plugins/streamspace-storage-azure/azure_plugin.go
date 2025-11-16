package main

import ("encoding/json"; "fmt"; "github.com/yourusername/streamspace/api/internal/plugins"; "github.com/Azure/azure-storage-blob-go/azblob")

type AzurePlugin struct {
	plugins.BasePlugin
	config AzureConfig
	client azblob.ContainerURL
}

type AzureConfig struct {
	Enabled        bool          `json:"enabled"`
	AccountName    string        `json:"accountName"`
	AccountKey     string        `json:"accountKey"`
	ContainerName  string        `json:"containerName"`
	Endpoint       string        `json:"endpoint"`
	StoragePaths   StoragePaths  `json:"storagePaths"`
}

type StoragePaths struct {
	Recordings string `json:"recordings"`
	Snapshots  string `json:"snapshots"`
	Uploads    string `json:"uploads"`
}

func (p *AzurePlugin) Initialize(ctx *plugins.PluginContext) error {
	configBytes, _ := json.Marshal(ctx.Config)
	json.Unmarshal(configBytes, &p.config)

	if !p.config.Enabled {
		ctx.Logger.Info("Azure Blob Storage is disabled")
		return nil
	}

	// Create credential
	credential, err := azblob.NewSharedKeyCredential(p.config.AccountName, p.config.AccountKey)
	if err != nil {
		return fmt.Errorf("failed to create Azure credentials: %w", err)
	}

	// Create pipeline
	pipeline := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	// Construct service URL
	endpoint := p.config.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.blob.core.windows.net", p.config.AccountName)
	}

	serviceURL, _ := url.Parse(endpoint)
	containerURL := azblob.NewContainerURL(*serviceURL, pipeline).NewContainerURL(p.config.ContainerName)

	p.client = containerURL

	ctx.Logger.Info("Azure Blob Storage initialized", "account", p.config.AccountName, "container", p.config.ContainerName)
	return nil
}

func (p *AzurePlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Azure Blob Storage plugin loaded")
	return nil
}

// UploadFile uploads a file to Azure Blob Storage
func (p *AzurePlugin) UploadFile(path string, data []byte) error {
	blobURL := p.client.NewBlockBlobURL(path)
	_, err := azblob.UploadBufferToBlockBlob(context.Background(), data, blobURL, azblob.UploadToBlockBlobOptions{})
	return err
}

// DownloadFile downloads a file from Azure Blob Storage
func (p *AzurePlugin) DownloadFile(path string) ([]byte, error) {
	blobURL := p.client.NewBlockBlobURL(path)
	downloadResponse, err := blobURL.Download(context.Background(), 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		return nil, err
	}

	bodyStream := downloadResponse.Body(azblob.RetryReaderOptions{})
	defer bodyStream.Close()
	return ioutil.ReadAll(bodyStream)
}

// DeleteFile deletes a file from Azure Blob Storage
func (p *AzurePlugin) DeleteFile(path string) error {
	blobURL := p.client.NewBlockBlobURL(path)
	_, err := blobURL.Delete(context.Background(), azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
	return err
}

func init() {
	plugins.Register("streamspace-storage-azure", &AzurePlugin{})
}
