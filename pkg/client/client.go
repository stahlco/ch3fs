package client

import (
	"fmt"
	"go.uber.org/zap"
)

type Client struct {
	Logger        *zap.SugaredLogger
	currentLeader string
}

func NewClient(logger *zap.SugaredLogger) *Client {

	return &Client{
		Logger:        logger,
		currentLeader: ch3fTarget,
	}
}

func (c *Client) UploadRandomRecipe() (string, error) {
	fileName := getFileName()
	content := getContent()
	res, err := c.UploadRecipe(c.currentLeader, fileName, content)
	if err != nil || res == nil {
		c.Logger.Errorf("Upload failed: %v", err)
		return "", err
	}
	if !res.Success {
		if res, err = c.UploadRecipe(res.LeaderContainer+":8080", fileName, content); err != nil || res == nil {
			c.Logger.Errorf("Upload to leader failed: %v", err)
			return "", err
		}
		if res.Success {
			c.Logger.Infof("Successfully processed request with filename: %s", fileName)
			c.currentLeader = res.LeaderContainer + ":8080"
		} else if !res.Success {
			c.Logger.Infof("Request for filename: %s was not successful Response: %v", fileName, res)
			return fileName, fmt.Errorf("request not successfull")
		}
	}

	return fileName, nil
}
