# OpenAI Assistant Integration

This project demonstrates the integration of the OpenAI Assistant feature and API in both Golang (CLI) and Python (API).

## Installation

Clone the repository and navigate to the project directory:

```bash
git clone <repository-url>
cd <project-directory>
```

Install the Go dependencies:

```
go mod download
```

Set up the Python environment:

```Bash
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
```

## Usage

### Golang CLI

You can run the Golang CLI project with the following commands:

```
go run main.go "Some question"
go run main.go "Some question" "thread_123"
go run main.go "Some question" "thread_123" "msg_AZS"
```

### Python API

To run the Python API project:

```
python3 main.py
```

In another terminal, you can send requests to the API:

```Bash
QUESTION="Some question" curl --location 'http://localhost:38234/api/v1/message' \
    --header 'Content-Type: application/json' \
    --data '{
    "question": "$QUESTION",
}'

QUESTION="Some question" curl --location 'http://localhost:38234/api/v1/message' \
    --header 'Content-Type: application/json' \
    --data '{
    "question": "$QUESTION",
    "thread_id": "thread_123",
}'

QUESTION="Some question" curl --location 'http://localhost:38234/api/v1/message' \
    --header 'Content-Type: application/json' \
    --data '{
    "question": "$QUESTION",
    "thread_id": "thread_123",
    "after_message_id": "msg_AZS"
}'
```

## Contributing

Contributions are welcome. Please make sure to update tests as appropriate.

## License

MIT