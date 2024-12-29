Below is a sample README.md that you could include in the root of your project. It explains the overall architecture (agent, backend, and frontend), how to build and run the containers, and how to use the system.

Docker Stats Monitoring

This project consists of three key parts:
	1.	Go Agent (runs on each server/host you want to monitor)
	•	Collects Docker image size in GB every hour and sends it to the backend.
	•	Performs a docker prune once a day automatically.
	•	Listens for manual POST /prune requests (triggered from the backend UI).
	2.	Go Backend (runs as a web service)
	•	Receives metrics from agents (POST /api/docker-stats).
	•	Stores the metrics in memory for up to 24 hours.
	•	Displays the current list of instances and their Docker stats on a simple HTML page (GET /).
	•	Triggers manual prune by calling the respective agent (POST /api/prune?instance=...), then removes that instance’s data from memory.
	3.	React Frontend
	•	Shows a dashboard of instances and their Docker image sizes.
	•	Allows manual docker prune by calling the backend endpoint, which in turn calls the agent.

Project Structure

.
├── docker-compose.yml         # Docker Compose file to build/run backend & frontend
├── README.md                  # This README
├── backend
│   ├── Dockerfile             # Multi-stage build for Go backend
│   ├── go.mod
│   ├── go.sum
│   └── main.go                # Go backend code
└── frontend
    ├── Dockerfile             # Multi-stage build for React app
    ├── package.json
    ├── package-lock.json
    ├── public
    └── src                    # React source code

The Go Agent is not included in the same Docker Compose setup because it typically runs directly on each host you want to monitor (outside this environment). You would build or deploy that separately on each machine.

Prerequisites
	•	Docker
	•	Docker Compose
	•	(Optional) Go if you want to build the backend locally without Docker.

How It Works
	1.	Go Agent (on each host)
	•	By default, it sends metrics to REMOTE_SERVER_URL (e.g. http://YOUR_BACKEND_HOST:3000/api/docker-stats).
	•	Once a day, it runs docker prune on its own.
	•	Exposes POST /prune on :8080 for manual pruning requests.
	2.	Go Backend
	•	Receives incoming JSON metrics at POST /api/docker-stats.
	•	Stores them in memory.
	•	Provides a simple HTML overview at GET / (on port 3000).
	•	Provides a POST /api/prune?instance={id} endpoint, which calls the agent’s http://{id}:8080/prune and then removes the instance’s data from memory.
	3.	React Frontend
	•	Runs in an Nginx container (by default on port 8080).
	•	Fetches data from the Go Backend (GET /api/docker-stats).
	•	Allows user to click “Prune” on a specific instance, which sends POST /api/prune?instance={id} to the backend.

Building and Running with Docker Compose
	1.	Clone or place this repository on your machine.
	2.	From the root directory (where docker-compose.yml is located), run:

docker-compose build


	3.	Then start the containers:

docker-compose up


	4.	You should see two services:
	•	web (Go backend) on localhost:3000
	•	frontend (React) on localhost:8080

Verify
	•	Backend: Open your browser at http://localhost:3000 to see a simple HTML table of instances (it may be empty at first).
	•	Frontend: Open http://localhost:8080 to see the React dashboard.

The Agent

The agent is not included in the Docker Compose, as it typically runs on each separate host you want to monitor. Steps to deploy the agent:
	1.	Build the agent binary (on the target server):

go build -o docker-agent agent.go

or cross-compile for that environment.

	2.	Run the agent (as a daemon, for example):

./docker-agent &

	•	Make sure you set REMOTE_SERVER_URL environment variable to point to your backend, e.g.

export REMOTE_SERVER_URL="http://YOUR_BACKEND_HOST:3000/api/docker-stats"


	•	The agent will listen on port 8080 for manual prune requests: POST /prune.

	3.	The agent automatically:
	•	Sends metrics to the backend every hour.
	•	Runs docker prune once a day on its host.
	•	Responds to manual POST /prune requests from the backend.

Endpoints Overview

Backend (Go):
	•	GET /
Returns a basic HTML table of known instances and their Docker image stats.
	•	POST /api/docker-stats
Receives JSON data from agents.
Example payload:

{
  "instance_id": "my-server-123",
  "images_size_gb": 3.72,
  "timestamp": "2024-01-01T12:00:00Z",
  "prune_action": false
}


	•	POST /api/prune?instance={ID}
Tells the backend to contact the agent at http://{ID}:8080/prune. On success, that instance’s stats are removed from memory.

Agent (Go):
	•	POST /prune
Triggers a docker prune on the host, then sends a follow-up metric to the backend with prune_action=true.
	•	GET /health
Health endpoint (optional).

React Frontend:
	•	Fetches data from GET /api/docker-stats (if you implement a GET route on the backend, or it may fetch from some other route you create).
	•	Calls POST /api/prune?instance={ID} for manual pruning.

Customizing
	•	Retention: The backend retains metrics for 24 hours by default, removing old entries. You can adjust this in main.go (the statsRetention constant).
	•	Automatic Prune Frequency: The agent prunes once per day; see pruneInterval = 24 * time.Hour in the agent code.
	•	Agent->Backend URL: The agent reads REMOTE_SERVER_URL from the environment or uses a default (http://localhost:3000/api/docker-stats).
	•	Service Ports: In docker-compose.yml, you can change ports: - "3000:3000" and - "8080:80" if needed.

Contributing
	•	Issues and pull requests are welcome.
	•	For significant changes, please open an issue first to discuss what you would like to change.

License

MIT License 