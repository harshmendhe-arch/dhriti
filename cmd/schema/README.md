# Dhriti Configuration Schema Generator

This tool generates a JSON Schema for the Dhriti configuration file. The schema can be used to validate configuration files and provide autocompletion in editors that support JSON Schema.

## Usage

```bash
go run cmd/schema/main.go > dhriti-schema.json
```

This will generate a JSON Schema file that can be used to validate configuration files.

## Schema Features

The generated schema includes:

- All configuration options with descriptions
- Default values where applicable
- Validation for enum values (e.g., model IDs, provider types)
- Required fields
- Type checking

## Using the Schema

You can use the generated schema in several ways:

1. **Editor Integration**: Many editors (VS Code, JetBrains IDEs, etc.) support JSON Schema for validation and autocompletion. You can configure your editor to use the generated schema for `.dhriti.json` files.

2. **Validation Tools**: You can use tools like [jsonschema](https://github.com/Julian/jsonschema) to validate your configuration files against the schema.

3. **Documentation**: The schema serves as documentation for the configuration options.

## Example Configuration

Here's an example configuration that conforms to the schema:

```json
{
  "data": {
    "directory": ".dhriti"
  },
  "debug": false,
  "providers": {
    "openai": {
      "apiKey": "your-api-key"
    }
  },
  "agents": {
    "coder": {
      "model": "gpt-4.1",
      "maxTokens": 5000,
      "reasoningEffort": "medium"
    },
    "task": {
      "model": "gpt-4.1-mini",
      "maxTokens": 5000
    },
    "title": {
      "model": "gpt-4.1-mini",
      "maxTokens": 80
    }
  }
}
```
