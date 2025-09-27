# Transform Test Application

This project demonstrates the usage of the `Transform` function from the `datastore` package. It initializes a datastore, loads sample JSON data, and applies transformations to the data.

## Project Structure

```
transform-test-app
├── main.go            # Entry point of the application
├── testdata
│   └── sample.json    # Sample JSON data for testing
├── go.mod             # Module definition
├── go.sum             # Module dependency checksums
└── README.md          # Project documentation
```

## Getting Started

### Prerequisites

- Go 1.16 or later
- IPFS Go Datastore library

### Installation

1. Clone the repository:
   ```
   git clone <repository-url>
   cd transform-test-app
   ```

2. Install dependencies:
   ```
   go mod tidy
   ```

### Running the Application

To run the application, execute the following command in the project directory:

```
go run main.go
```

### Sample Data

The application uses sample JSON data located in `testdata/sample.json`. Ensure that this file contains valid JSON data that can be transformed using the `Transform` function.

### Transform Function

The `Transform` function processes data stored in the datastore, applying specified transformations based on the provided parameters. It supports JSON extraction and patching using jq-like syntax.

### License

This project is licensed under the MIT License. See the LICENSE file for details.