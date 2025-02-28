package cmd

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"github.com/lib/pq"
)

import cloudevents "github.com/cloudevents/sdk-go/v2"

type cloudeventHandler struct {
	Callback func(event *cloudevents.Event)
}

var cloudeventHandlers []cloudeventHandler = []cloudeventHandler{}

func setupCloudevents(ctx context.Context, conn *sql.Conn) (err error) {

	cloudEventConsumer := func(notice *pq.Error) {
		event := cloudevents.NewEvent()
		err := json.Unmarshal([]byte(notice.Message), &event)
		if err == nil {
			for _, handler := range cloudeventHandlers {
				handler.Callback(&event)
			}
		}
	}
	err = conn.Raw(func(driverConn any) error {
		pq.SetNoticeHandler(driverConn.(driver.Conn), cloudEventConsumer)
		return nil
	})
	if err != nil {
		return err
	}

	var rows *sql.Rows
	rows, err = conn.QueryContext(ctx, `select omni_cloudevents.create_notice_publisher(publish_uncommitted => true)`)
	if err == nil {
		for rows.Next() {
		}
	}

	return
}
