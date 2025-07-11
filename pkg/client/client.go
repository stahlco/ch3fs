package client

import (
	"fmt"
	"go.uber.org/zap"
	"time"
)

type Client struct {
	Logger        *zap.SugaredLogger
	RoundRobin    *RoundRobin
	currentLeader string
}

func NewClient(logger *zap.SugaredLogger) *Client {
	cluster, err := discoverServers(logger)
	if err != nil {
		logger.Errorf("Not able to find servers in the cluster, error: %v", err)
		return nil
	}
	rr := NewRoundRobin(cluster)
	return &Client{
		Logger:        logger,
		RoundRobin:    rr,
		currentLeader: "",
	}
}

func (c *Client) UploadRandomRecipe() (string, error) {
	fileName := getFileName()
	content := getContent()

	// Initialize leader if not set
	if c.currentLeader == "" {
		c.currentLeader = c.RoundRobin.Next()
	}

	for {
		res, err := c.UploadRecipe(c.currentLeader, fileName, content)
		if err != nil {
			c.Logger.Errorf("Upload failed: %v", err)
			return "", err
		}
		if res == nil {
			c.Logger.Warn("Upload returned nil response")
			return "", fmt.Errorf("upload response is nil")
		}

		if res.Success {
			c.Logger.Infof("Successfully uploaded file: %s", fileName)
			return fileName, nil
		}

		// Not successful: retry with new leader
		newLeader := res.LeaderContainer + ":8080"
		c.Logger.Infof("Retrying with new leader: %s", newLeader)

		res, err = c.UploadRecipe(newLeader, fileName, content)
		if err != nil || res == nil {
			c.Logger.Errorf("Retry to new leader failed: %v", err)
			return "", err
		}

		if res.Success {
			c.Logger.Infof("Successfully uploaded file after retry: %s", fileName)
			c.currentLeader = newLeader
			return fileName, nil
		}

		// Even retry failed – backoff and try again
		c.Logger.Warnf("Retry failed with leader: %s. Will backoff and try next.", newLeader)
		c.currentLeader = c.RoundRobin.Next()
		time.Sleep(time.Duration(BackoffWithJitter(50.0)) * time.Millisecond)
	}
}
