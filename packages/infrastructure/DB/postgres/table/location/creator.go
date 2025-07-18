package locationtable

import (
	Error "sentinel/packages/common/errors"
	LocationDTO "sentinel/packages/core/location/DTO"
	"sentinel/packages/infrastructure/DB/postgres/connection"
	"sentinel/packages/infrastructure/DB/postgres/executor"
	"sentinel/packages/infrastructure/DB/postgres/query"

	"github.com/google/uuid"
)

func (_ *Manager) SaveLocation(dto *LocationDTO.Full) *Error.Status {
	insertQuery := query.New(
		`INSERT INTO "location" (id, ip, session_id, country, region, city, latitude, longitude, isp, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);`,
		uuid.NewString(),
		dto.IP,
		dto.SessionID,
		dto.Country,
		dto.Region,
		dto.City,
		dto.Latitude,
		dto.Longitude,
		dto.ISP,
		dto.CreatedAt,
	)

	return executor.Exec(connection.Primary, insertQuery)
}

