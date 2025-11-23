#!/bin/bash
TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiYWRtaW4iLCJ1c2VybmFtZSI6ImFkbWluIiwiZW1haWwiOiJhZG1pbkBzdHJlYW1zcGFjZS5sb2NhbCIsInJvbGUiOiJhZG1pbiIsImdyb3VwcyI6WyJhbGwtdXNlcnMiXSwiaXNzIjoic3RyZWFtc3BhY2UtYXBpIiwic3ViIjoiYWRtaW4iLCJleHAiOjE3NjM4NDA3MzgsIm5iZiI6MTc2Mzc1NDMzOCwiaWF0IjoxNzYzNzU0MzM4LCJqdGkiOiJiYjFhNjFkZjlkMmFjNTJjYzY2OTM0YWZjMjk1MWVlODk2ZmUwMThmN2MzMDIzYzM4Y2ZhZjM3MTQ4YmM3MzU2In0.CRkxANqeNKCARxOQTVC9pyhP4pM6VJPxQ4KFb5EH0Ag"

curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}' | jq .
