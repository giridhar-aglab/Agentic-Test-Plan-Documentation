# Test Plan — gin-gonic/gin

## Summary

- **231 scenarios** covering **40 of 40** prioritised risks
- Priority mix: 74 P0, 157 P1
- Structural validation passed: every scenario has steps, a checkable expected result, and a source reference that resolves in this commit
- 44 gaps recorded; see the end of this document

## Run metadata

- Run: `run-1788172554`
- Commit: `working tree`
- Languages: go
- Test framework: testify
- Analysis depth: 57 syntactic
- Degraded tools: `code_parse_go`, `repo_read_file`

## Scope

130 files in the commit, 58 selected for analysis.

| File | Language | Size |
|---|---|---|
| `auth.go` | go | 3838 B |
| `binding/binding.go` | go | 4294 B |
| `binding/binding_nomsgpack.go` | go | 3822 B |
| `binding/bson.go` | go | 580 B |
| `binding/default_validator.go` | go | 2342 B |
| `binding/form.go` | go | 1357 B |
| `binding/form_mapping.go` | go | 13602 B |
| `binding/header.go` | go | 868 B |
| `binding/json.go` | go | 1543 B |
| `binding/msgpack.go` | go | 768 B |
| `binding/multipart_form_mapping.go` | go | 2244 B |
| `binding/plain.go` | go | 868 B |
| `binding/protobuf.go` | go | 923 B |
| `binding/query.go` | go | 460 B |
| `binding/toml.go` | go | 698 B |
| `binding/uri.go` | go | 399 B |
| `binding/xml.go` | go | 677 B |
| `binding/yaml.go` | go | 691 B |
| `codec/json/api.go` | go | 1942 B |
| `codec/json/go_json.go` | go | 898 B |
| `codec/json/json.go` | go | 919 B |
| `codec/json/jsoniter.go` | go | 985 B |
| `codec/json/sonic.go` | go | 953 B |
| `context.go` | go | 48274 B |
| `context_appengine.go` | go | 261 B |
| `debug.go` | go | 3000 B |
| `deprecated.go` | go | 762 B |
| `doc.go` | go | 414 B |
| `errors.go` | go | 3944 B |
| `fs.go` | go | 1396 B |
| `gin.go` | go | 27227 B |
| `ginS/gins.go` | go | 5654 B |
| `internal/bytesconv/bytesconv.go` | go | 719 B |
| `internal/fs/fs.go` | go | 362 B |
| `logger.go` | go | 8070 B |
| `mode.go` | go | 2456 B |
| `path.go` | go | 4853 B |
| `recovery.go` | go | 5816 B |
| `render/bson.go` | go | 790 B |
| `render/data.go` | go | 742 B |
| `render/html.go` | go | 2875 B |
| `render/json.go` | go | 5013 B |
| `render/msgpack.go` | go | 1175 B |
| `render/pdf.go` | go | 651 B |
| `render/protobuf.go` | go | 852 B |
| `render/reader.go` | go | 1195 B |
| `render/redirect.go` | go | 904 B |
| `render/render.go` | go | 1203 B |
| `render/text.go` | go | 1091 B |
| `render/toml.go` | go | 820 B |
| `render/xml.go` | go | 730 B |
| `render/yaml.go` | go | 821 B |
| `response_writer.go` | go | 3381 B |
| `routergroup.go` | go | 9279 B |
| `test_helpers.go` | go | 1902 B |
| `tree.go` | go | 24381 B |
| `utils.go` | go | 4117 B |
| `version.go` | go | 249 B |

**Excluded:** 1 generated code, 40 test file, 31 unsupported language.

**Existing tests:** none found. Every scenario below is new work.

**Dependencies:** 16 direct, 35 total.

## Test considerations

### Risk register

| Risk | Component | Level | Score | Existing tests | Why |
|---|---|---|---|---|---|
| `RISK-response-writer` | response_writer | **critical** | 47.9 | none | response_writer has 0 branches across 0 exported symbols and 5 distinct failure paths, with no tests in its package. |
| `RISK-binding-form` | binding/form | **critical** | 39.8 | none | binding/form has 0 branches across 0 exported symbols and 4 distinct failure paths, with no tests in its package. |
| `RISK-gin` | gin | **critical** | 39.2 | none | gin has 0 branches across 0 exported symbols and 5 distinct failure paths, with no tests in its package. |
| `RISK-json-go-json` | json/go_json | **critical** | 39.2 | none | json/go_json has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-json-sonic` | json/sonic | **critical** | 38.5 | none | json/sonic has 0 branches across 0 exported symbols and 5 distinct failure paths, with no tests in its package. |
| `RISK-json-jsoniter` | json/jsoniter | **critical** | 36.5 | none | json/jsoniter has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-render-json` | render/json | **critical** | 36.5 | none | render/json has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-json-json` | json/json | **critical** | 35.8 | none | json/json has 0 branches across 0 exported symbols and 5 distinct failure paths, with no tests in its package. |
| `RISK-binding-form-mapping` | binding/form_mapping | **critical** | 35.1 | none | binding/form_mapping has 0 branches across 0 exported symbols and 5 distinct failure paths, with no tests in its package. |
| `RISK-routergroup` | routergroup | **critical** | 33.8 | none | routergroup has 0 branches across 0 exported symbols and 4 distinct failure paths, with no tests in its package. |
| `RISK-render-html` | render/html | **critical** | 32.4 | none | render/html has 0 branches across 0 exported symbols and 4 distinct failure paths, with no tests in its package. |
| `RISK-utils` | utils | **critical** | 31.7 | none | utils has 0 branches across 0 exported symbols and 5 distinct failure paths, with no tests in its package. |
| `RISK-auth` | auth | **critical** | 31.1 | none | auth has 0 branches across 0 exported symbols and 4 distinct failure paths, with no tests in its package. |
| `RISK-logger` | logger | **high** | 28.4 | none | logger has 0 branches across 0 exported symbols and 4 distinct failure paths, with no tests in its package. |
| `RISK-tree` | tree | **high** | 28.4 | none | tree has 0 branches across 0 exported symbols and 6 distinct failure paths, with no tests in its package. |
| `RISK-gins-gins` | ginS/gins | **high** | 27.0 | none | ginS/gins has 0 branches across 0 exported symbols and 5 distinct failure paths, with no tests in its package. |
| `RISK-debug` | debug | **high** | 26.3 | none | debug has 0 branches across 0 exported symbols and 4 distinct failure paths, with no tests in its package. |
| `RISK-binding-default-validator` | binding/default_validator | **high** | 25.0 | none | binding/default_validator has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-render-reader` | render/reader | **high** | 25.0 | none | render/reader has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-binding-binding` | binding/binding | **high** | 23.6 | none | binding/binding has 0 branches across 0 exported symbols and 4 distinct failure paths, with no tests in its package. |
| `RISK-errors` | errors | **high** | 23.6 | none | errors has 0 branches across 0 exported symbols and 4 distinct failure paths, with no tests in its package. |
| `RISK-recovery` | recovery | **high** | 23.6 | none | recovery has 0 branches across 0 exported symbols and 4 distinct failure paths, with no tests in its package. |
| `RISK-test-helpers` | test_helpers | **high** | 23.6 | none | test_helpers has 0 branches across 0 exported symbols and 4 distinct failure paths, with no tests in its package. |
| `RISK-binding-json` | binding/json | **high** | 22.3 | none | binding/json has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-binding-msgpack` | binding/msgpack | **high** | 22.3 | none | binding/msgpack has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-binding-xml` | binding/xml | **high** | 22.3 | none | binding/xml has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-binding-binding-nomsgpack` | binding/binding_nomsgpack | **high** | 21.6 | none | binding/binding_nomsgpack has 0 branches across 0 exported symbols and 5 distinct failure paths, with no tests in its package. |
| `RISK-binding-toml` | binding/toml | **high** | 21.6 | none | binding/toml has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-binding-yaml` | binding/yaml | **high** | 21.6 | none | binding/yaml has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-mode` | mode | **high** | 21.6 | none | mode has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-render-protobuf` | render/protobuf | **high** | 21.6 | none | render/protobuf has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-binding-multipart-form-mapping` | binding/multipart_form_mapping | **high** | 20.3 | none | binding/multipart_form_mapping has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-binding-plain` | binding/plain | **high** | 20.3 | none | binding/plain has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-fs` | fs | **high** | 20.3 | none | fs has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-binding-header` | binding/header | **high** | 18.9 | none | binding/header has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-binding-query` | binding/query | **high** | 18.9 | none | binding/query has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-render-bson` | render/bson | **high** | 18.9 | none | render/bson has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-render-msgpack` | render/msgpack | **high** | 18.9 | none | render/msgpack has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-render-xml` | render/xml | **high** | 18.9 | none | render/xml has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-render-text` | render/text | **high** | 18.2 | none | render/text has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-binding-protobuf` | binding/protobuf | **medium** | 17.6 | none | binding/protobuf has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-deprecated` | deprecated | **medium** | 17.6 | none | deprecated has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-json-api` | json/api | **medium** | 16.9 | none | json/api has 0 branches across 0 exported symbols and 5 distinct failure paths, with no tests in its package. |
| `RISK-render-data` | render/data | **medium** | 16.9 | none | render/data has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-render-toml` | render/toml | **medium** | 16.9 | none | render/toml has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-binding-uri` | binding/uri | **medium** | 16.2 | none | binding/uri has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-path` | path | **medium** | 15.5 | none | path has 0 branches across 0 exported symbols and 3 distinct failure paths, with no tests in its package. |
| `RISK-binding-bson` | binding/bson | **medium** | 14.2 | none | binding/bson has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-render-render` | render/render | **medium** | 14.2 | none | render/render has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-render-yaml` | render/yaml | **medium** | 14.2 | none | render/yaml has 0 branches across 0 exported symbols and 2 distinct failure paths, with no tests in its package. |
| `RISK-render-pdf` | render/pdf | **medium** | 10.8 | none | render/pdf has 0 branches across 0 exported symbols and 1 distinct failure paths, with no tests in its package. |
| `RISK-render-redirect` | render/redirect | **medium** | 10.8 | none | render/redirect has 0 branches across 0 exported symbols and 1 distinct failure paths, with no tests in its package. |
| `RISK-fs-fs` | fs/fs | **medium** | 8.1 | none | fs/fs has 0 branches across 0 exported symbols and 1 distinct failure paths, with no tests in its package. |
| `RISK-doc` | doc | **low** | 7.4 | none | doc has 0 branches across 0 exported symbols, with no tests in its package. |
| `RISK-context-appengine` | context_appengine | **low** | 2.7 | none | context_appengine has 0 branches across 0 exported symbols, with no tests in its package. |
| `RISK-bytesconv-bytesconv` | bytesconv/bytesconv | **low** | 0.0 | none | bytesconv/bytesconv has 0 branches across 0 exported symbols, with no tests in its package. |
| `RISK-version` | version | **low** | 0.0 | none | version has 0 branches across 0 exported symbols, with no tests in its package. |

_Deterministic scoring: branches ×1.0, error paths ×2.5, external calls ×2.0, side effects ×2.0, exported symbols ×0.5; ×1.8 when untested, ×0.75 when analysis was syntactic only._

### Testability obstacles

Dependencies a test must stub or cross:

- `External HTTP requests` — To test the behavior of the web framework.
- `Validator.ValidateStruct` — To test scenarios where validation is successful or fails.
- `bson.Marshal` — To verify BSON marshaling behavior during tests.
- `bson.Unmarshal` — To test BSON unmarshalling without actual data.
- `bytesconv.StringToBytes` — To control the byte conversion behavior.
- `c.AbortWithStatus` — To simulate aborting the handler chain with specific status.
- `c.MustBindWith` — To test the binding behavior without executing the actual binding logic.
- `c.requestHeader` — To simulate request headers for testing credentials.
- `codec.NewDecoder` — To control the decoding process and simulate error scenarios.
- `codec.NewEncoder` — It creates an encoder for MessagePack encoding.
- `debugPrint` — To capture debug logs for environment variable messages.
- `flag.Lookup` — to check if running in test mode
- `fmt.Fprintf` — It performs outputs that can be intercepted during tests.
- `gin.Default()` — To ensure that tests can run against a controlled Gin engine.
- `group.engine.addRoute` — To test route registration behavior.
- `http.Client.Get` — To simulate server responses during tests.
- `http.CloseNotifier.CloseNotify` — To handle connection close notification.
- `http.FileServer` — To test static file serving logic.
- `http.FileSystem.Open` — to simulate file opening behavior
- `http.Flusher.Flush` — To handle flushing.
- `http.Hijacker.Hijack` — To handle hijacking.
- `http.Pusher` — To handle HTTP/2 server push.
- `http.Redirect` — To simulate HTTP responses in tests.
- `http.Request` — The component relies on http.Request for accessing multipart form data.
- `http.Request.Body` — To simulate various request body contents in tests.
- `http.ResponseWriter` — It is used for writing HTTP responses.
- `http.ResponseWriter.Write` — To verify body writing behavior.
- `http.ResponseWriter.WriteHeader` — To verify header writing behavior.
- `json.API.Marshal` — Needed to simulate JSON marshaling behavior and errors.
- `json.API.MarshalIndent` — Needed to simulate indented JSON marshaling behavior and errors.
- `json.API.NewDecoder` — It is a dependency for JSON decoding.
- `json.API.Unmarshal` — This is used to decode input into specified structures.
- `json.Marshal` — Must mock to simulate marshaling errors.
- `json.MarshalIndent` — Must mock to simulate marshaling with indentation errors.
- `json.NewDecoder` — Must mock to simulate decoder creation.
- `json.NewEncoder` — Must mock to simulate encoder creation.
- `json.Unmarshal` — Must mock to simulate unmarshaling errors.
- `mapForm` — It is called to map the query parameters to the object.
- `mapHeader` — to simulate header binding errors
- `mapURI` — It's used to map URI values to the object.
- `mappingByPtr` — To simulate mapping form data when handling multipart requests.
- `net.Listen` — To avoid binding to actual network interfaces during tests.
- `os.File` — To validate file output handling
- `os.Getenv` — To test behavior when the environment variable PORT is defined or undefined.
- `os.Open` — To test file reading errors
- `os.Remove` — To avoid deleting files during tests.
- `os.Stdout` — To test logging to standard output
- `proto.Marshal` — It is a third-party call that could fail.
- `proto.Unmarshal` — It needs to be mocked to test the unmarshalling behavior.
- `req.ParseForm` — To simulate form parsing in tests.
- `req.ParseMultipartForm` — To simulate multipart form parsing in tests.
- `sonic.Marshal` — To simulate JSON encoding scenarios.
- `sonic.NewDecoder` — To test decoder creation.
- `sonic.NewEncoder` — To test encoder creation.
- `sonic.Unmarshal` — To simulate JSON decoding scenarios.
- `template.JSEscapeString` — To control the behavior of escaping JavaScript strings.
- `toml.NewDecoder` — To mock TOML decoding behavior for tests.
- `v.validate.Struct(obj)` — to validate the struct and check for errors
- `validate` — To simulate validation errors
- `validator.New()` — to ensure controlled behavior of the validator during tests
- `w.Write` — It writes to the HTTP response and could fail.
- `writeContentType` — It is called to set the content type of the response.
- `xml.NewDecoder` — To test the XML decoding behavior.
- `xml.NewEncoder` — to mock XML encoding behavior
- `yaml.NewDecoder` — To simulate decoding errors

Side effects that need isolation:

- **auth** — Sets headers for unauthorized responses; Modifies context with user information
- **binding/binding** — Mutates request data based on binding; May call validation on provided struct
- **binding/bson** — Reads from the HTTP request body
- **binding/default_validator** — initializes a validator instance; modifies the state of the defaultValidator instance
- **binding/form** — Modifies the 'obj' parameter by mapping form data to it
- **binding/form_mapping** — Mutation of input structures; Modification of maps
- **binding/header** — modifies the obj argument by binding HTTP headers
- **binding/json** — Mutates the 'obj' parameter with the decoded JSON data
- **binding/msgpack** — Mutates the object passed as a parameter after decoding
- **binding/multipart_form_mapping** — Sets fields in structs via reflection
- **binding/plain** — Mutates the passed object based on the request body
- **binding/protobuf** — Reads from the HTTP request body
- **binding/query** — Modifies the provided object based on query parameters
- **binding/toml** — Reads from the request body; Decodes TOML data
- **binding/xml** — Mutates the object passed as an argument to Bind and BindBody functions
- **binding/yaml** — Reads from request body; Reads from byte slice
- **context_appengine** — Sets defaultPlatform variable to PlatformGoogleAppEngine
- **debug** — Writes debug information to output streams; Modifies global debug state based on mode
- **deprecated** — Logs a deprecation warning
- **doc** — Starts an HTTP server on port 8080.
- **errors** — Creates JSON representation of errors; Writes to buffer in String method
- **fs** — None
- **gin** — Writes to HTTP response; Starts HTTP server; Modifies request path for redirection
- **ginS/gins** — Potentially starts an HTTP server; Registers routes in the Gin engine
- **json/go_json** — Writes to io.Writer in NewEncoder; Reads from io.Reader in NewDecoder
- **json/json** — May write to the provided io.Writer during encoding; Read from the provided io.Reader during decoding
- **json/jsoniter** — Instantiation of jsoniter API during init
- **json/sonic** — None
- **logger** — Writes log messages to the specified output writer; Mutates context with log parameters
- **mode** — modifies global state for gin mode; sets output writers for debug and error logging
- **path** — Mutates the input string by generating cleaned versions; Allocates memory for buffers when necessary
- **recovery** — Logs error messages to provided writer; Modifies context state on error
- **render/bson** — Writes BSON data to HTTP response
- **render/data** — Writes HTTP response headers; Writes response body data
- **render/html** — Writes HTTP response; Modifies HTTP headers for Content-Type
- **render/json** — Writes data to http.ResponseWriter with JSON content types
- **render/msgpack** — Writes to HTTP response writer
- **render/pdf** — Writes PDF data to the provided ResponseWriter
- **render/protobuf** — Writes data to HTTP response; Writes content type header to HTTP response
- **render/reader** — Writes headers to HTTP response; Copies content from Reader to HTTP response
- **render/redirect** — Modifies the HTTP response by setting a redirect status and location
- **render/render** — Modifies the HTTP response header
- **render/text** — Writes to http.ResponseWriter
- **render/toml** — Writes TOML data to HTTP response
- **render/xml** — writing XML output to the response writer
- **render/yaml** — Writes response with YAML content type
- **response_writer** — Writes the HTTP response header and body when Write is called
- **routergroup** — Modifies the group's middleware chain; Registers routes with the engine; Writes HTTP responses; May panic on invalid configurations
- **test_helpers** — Initializes a ResponseWriter in the context; Performs HTTP requests to check server readiness
- **tree** — panics on errors
- **utils** — Panic on invalid inputs in various functions; Setting a binding key in context when Bind function is successful

## Scenario catalogue

### auth

#### `TS-auth-001` — BasicAuthForRealm rejects an empty list of authorized credentials

**P0** · negative · covers `RISK-auth` · [`BasicAuthForRealm` in auth.go](https://github.com/gin-gonic/gin/blob/HEAD/auth.go)

*Steps:*

1. Call BasicAuthForRealm with an empty map of accounts.
2. Invoke the returned handler with a mock context.

*Expected:* The handler returns a 500 status with the response body containing 'Empty list of authorized credentials'.

#### `TS-auth-002` — BasicAuthForRealm rejects a user that is empty

**P0** · negative · covers `RISK-auth` · [`BasicAuthForRealm` in auth.go](https://github.com/gin-gonic/gin/blob/HEAD/auth.go)

*Steps:*

1. Call BasicAuthForRealm with a non-empty map of accounts containing an empty user name.
2. Invoke the returned handler with a mock context that simulates an authorization request.

*Expected:* The handler returns a 500 status with the response body containing 'User can not be empty'.

#### `TS-auth-003` — BasicAuthForRealm returns 401 for incorrect credentials

**P0** · negative · covers `RISK-auth` · [`BasicAuthForRealm` in auth.go](https://github.com/gin-gonic/gin/blob/HEAD/auth.go)

*Steps:*

1. Call BasicAuthForRealm with valid credentials in the accounts map.
2. Invoke the returned handler with a mock context containing invalid credentials.

*Expected:* The handler returns a 401 status with the WWW-Authenticate header set.

#### `TS-auth-004` — BasicAuthForRealm sets user in context for valid credentials

**P0** · unit · covers `RISK-auth` · [`BasicAuthForRealm` in auth.go](https://github.com/gin-gonic/gin/blob/HEAD/auth.go)

*Steps:*

1. Call BasicAuthForRealm with valid credentials in the accounts map.
2. Invoke the returned handler with a mock context containing valid credentials.

*Expected:* The user is set in the context with the key AuthUserKey.

#### `TS-auth-005` — BasicAuthForProxy rejects an empty list of authorized credentials

**P0** · negative · covers `RISK-auth` · [`BasicAuthForProxy` in auth.go](https://github.com/gin-gonic/gin/blob/HEAD/auth.go)

*Steps:*

1. Call BasicAuthForProxy with an empty map of accounts.
2. Invoke the returned handler with a mock context.

*Expected:* The handler returns a 500 status with the response body containing 'Empty list of authorized credentials'.

#### `TS-auth-006` — BasicAuthForProxy rejects a user that is empty

**P0** · negative · covers `RISK-auth` · [`BasicAuthForProxy` in auth.go](https://github.com/gin-gonic/gin/blob/HEAD/auth.go)

*Steps:*

1. Call BasicAuthForProxy with a non-empty map of accounts containing an empty user name.
2. Invoke the returned handler with a mock context that simulates a proxy authorization request.

*Expected:* The handler returns a 500 status with the response body containing 'User can not be empty'.

#### `TS-auth-007` — BasicAuthForProxy returns 407 for incorrect proxy credentials

**P0** · negative · covers `RISK-auth` · [`BasicAuthForProxy` in auth.go](https://github.com/gin-gonic/gin/blob/HEAD/auth.go)

*Steps:*

1. Call BasicAuthForProxy with valid credentials in the accounts map.
2. Invoke the returned handler with a mock context containing invalid proxy credentials.

*Expected:* The handler returns a 407 status with the Proxy-Authenticate header set.

#### `TS-auth-008` — BasicAuthForProxy sets proxy user in context for valid proxy credentials

**P0** · unit · covers `RISK-auth` · [`BasicAuthForProxy` in auth.go](https://github.com/gin-gonic/gin/blob/HEAD/auth.go)

*Steps:*

1. Call BasicAuthForProxy with valid credentials in the accounts map.
2. Invoke the returned handler with a mock context containing valid proxy credentials.

*Expected:* The proxy user is set in the context with the key AuthProxyUserKey.

### binding/binding

#### `TS-binding-binding-001` — Binding process fails when provided struct is nil

**P1** · negative · covers `RISK-binding-binding` · [`Default` in binding/binding.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding.go)

*Steps:*

1. Call Default with any HTTP method and content type
2. Pass a nil struct as the second parameter

*Expected:* An error indicating that the struct is nil is returned.

#### `TS-binding-binding-002` — Request method unsupported results in an error

**P1** · negative · covers `RISK-binding-binding` · [`Default` in binding/binding.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding.go)

*Steps:*

1. Call Default with an unsupported HTTP method and a valid content type

*Expected:* An error indicating the request method is unsupported is returned.

#### `TS-binding-binding-003` — Unsupported content type results in an error

**P1** · negative · covers `RISK-binding-binding` · [`Default` in binding/binding.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding.go)

*Steps:*

1. Call Default with a valid HTTP method and an unsupported content type

*Expected:* An error indicating the content type is unsupported is returned.

#### `TS-binding-binding-004` — Validator is nil results in no validation errors

**P1** · negative · covers `RISK-binding-binding` · [`validate` in binding/binding.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding.go)

*Preconditions:*

- Set Validator to nil

*Steps:*

1. Call validate with a valid struct

*Expected:* No error is returned during validation.

#### `TS-binding-binding-005` — Return appropriate Binding instance for each content type

**P1** · unit · covers `RISK-binding-binding` · [`Default` in binding/binding.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding.go)

*Steps:*

1. Call Default with a valid HTTP method and supported content types
2. Check that the returned Binding instance matches the expected Binding for each content type

*Expected:* The correct Binding instance is returned for each supported content type.

#### `TS-binding-binding-006` — validate successfully validates a valid struct

**P1** · unit · covers `RISK-binding-binding` · [`validate` in binding/binding.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding.go)

*Preconditions:*

- Validator is set and configured properly.

*Steps:*

1. Create a valid struct according to defined validation rules.
2. Call validate with this struct.

*Expected:* Function returns nil, indicating validation success.

### binding/binding_nomsgpack

#### `TS-binding-binding-nomsgpack-001` — Validator is nil during validation

**P1** · negative · covers `RISK-binding-binding-nomsgpack` · [`validate` in binding/binding_nomsgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding_nomsgpack.go)

*Steps:*

1. Call validate function with a struct object while the Validator variable is nil.

*Expected:* The function should return nil without panic.

#### `TS-binding-binding-nomsgpack-002` — HTTP method is not matching expected

**P1** · negative · covers `RISK-binding-binding-nomsgpack` · [`Default` in binding/binding_nomsgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding_nomsgpack.go)

*Steps:*

1. Call Default function with an unsupported HTTP method and a valid content type.

*Expected:* The function should return the Form binding instance.

#### `TS-binding-binding-nomsgpack-003` — Unsupported content type

**P1** · negative · covers `RISK-binding-binding-nomsgpack` · [`Default` in binding/binding_nomsgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding_nomsgpack.go)

*Steps:*

1. Call Default function with a valid HTTP method and an unsupported content type.

*Expected:* The function should return the Form binding instance.

#### `TS-binding-binding-nomsgpack-004` — Error during binding from request

**P1** · negative · covers `RISK-binding-binding-nomsgpack` · [`Bind` in binding/binding_nomsgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding_nomsgpack.go)

*Preconditions:*

- Mock HTTP request to simulate binding error.

*Steps:*

1. Call the Bind method on a binding instance that is expected to fail.

*Expected:* The method should return an error.

#### `TS-binding-binding-nomsgpack-005` — Error during validation of bound data

**P1** · negative · covers `RISK-binding-binding-nomsgpack` · [`validate` in binding/binding_nomsgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding_nomsgpack.go)

*Preconditions:*

- Set Validator to a mock that returns an error.

*Steps:*

1. Call validate function with bound data that is known to be invalid.

*Expected:* The function should return an error describing validation failure.

#### `TS-binding-binding-nomsgpack-006` — Default returns YAML binding for application/x-yaml content type

**P1** · unit · covers `RISK-binding-binding-nomsgpack` · [`Default` in binding/binding_nomsgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding_nomsgpack.go)

*Steps:*

1. Call Default with method 'POST' and contentType 'application/x-yaml'.

*Expected:* The result should be an instance of the YAML binding.

#### `TS-binding-binding-nomsgpack-007` — Default returns FormMultipart binding for multipart/form-data content type

**P1** · unit · covers `RISK-binding-binding-nomsgpack` · [`Default` in binding/binding_nomsgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding_nomsgpack.go)

*Steps:*

1. Call Default with method 'POST' and contentType 'multipart/form-data'.

*Expected:* The result should be an instance of the FormMultipart binding.

#### `TS-binding-binding-nomsgpack-008` — Default returns TOML binding for application/toml content type

**P1** · unit · covers `RISK-binding-binding-nomsgpack` · [`Default` in binding/binding_nomsgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding_nomsgpack.go)

*Steps:*

1. Call Default with method 'POST' and contentType 'application/toml'.

*Expected:* The result should be an instance of the TOML binding.

#### `TS-binding-binding-nomsgpack-009` — Default returns BSON binding for application/bson content type

**P1** · unit · covers `RISK-binding-binding-nomsgpack` · [`Default` in binding/binding_nomsgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding_nomsgpack.go)

*Steps:*

1. Call Default with method 'POST' and contentType 'application/bson'.

*Expected:* The result should be an instance of the BSON binding.

#### `TS-binding-binding-nomsgpack-010` — validate returns nil when Validator is nil

**P1** · unit · covers `RISK-binding-binding-nomsgpack` · [`validate` in binding/binding_nomsgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding_nomsgpack.go)

*Steps:*

1. Call validate with a dummy object when Validator is nil.

*Expected:* The result should be nil.

#### `TS-binding-binding-nomsgpack-011` — validate returns error on failed validation

**P1** · unit · covers `RISK-binding-binding-nomsgpack` · [`validate` in binding/binding_nomsgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding_nomsgpack.go)

*Steps:*

1. Set Validator to a mock that returns an error.
2. Call validate with a dummy object.

*Expected:* The result should be the error returned by the mock.

#### `TS-binding-binding-nomsgpack-012` — validate skips validation for non-struct types

**P1** · unit · covers `RISK-binding-binding-nomsgpack` · [`validate` in binding/binding_nomsgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding_nomsgpack.go)

*Steps:*

1. Set Validator to a mock.
2. Call validate with a non-struct type.

*Expected:* The result should be nil.

#### `TS-binding-binding-nomsgpack-013` — validate does not panic when type is not a struct

**P1** · unit · covers `RISK-binding-binding-nomsgpack` · [`validate` in binding/binding_nomsgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding_nomsgpack.go)

*Steps:*

1. Set Validator to a mock.
2. Call validate with a nil value.

*Expected:* The result should be nil.

#### `TS-binding-binding-nomsgpack-014` — Error when binding with unsupported content type

**P1** · negative · covers `RISK-binding-binding-nomsgpack` · [`Default` in binding/binding_nomsgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/binding_nomsgpack.go)

*Steps:*

1. Call Default with method 'POST' and an unsupported content type.

*Expected:* The result should be an instance of the Form binding.

### binding/default_validator

#### `TS-binding-default-validator-001` — ValidateStruct returns nil when input obj is nil

**P1** · unit · covers `RISK-binding-default-validator` · [`ValidateStruct` in binding/default_validator.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/default_validator.go)

*Steps:*

1. Create an instance of defaultValidator
2. Call ValidateStruct with nil as the input

*Expected:* The result should be nil.

#### `TS-binding-default-validator-002` — ValidateStruct returns nil when input obj is a valid struct

**P1** · unit · covers `RISK-binding-default-validator` · [`ValidateStruct` in binding/default_validator.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/default_validator.go)

*Steps:*

1. Create an instance of defaultValidator
2. Define a valid struct according to validation rules
3. Call ValidateStruct with the valid struct as the input

*Expected:* The result should be nil.

#### `TS-binding-default-validator-003` — ValidateStruct returns an error when input obj is a pointer to an invalid struct

**P1** · unit · covers `RISK-binding-default-validator` · [`ValidateStruct` in binding/default_validator.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/default_validator.go)

*Steps:*

1. Create an instance of defaultValidator
2. Define an invalid struct according to validation rules
3. Call ValidateStruct with a pointer to the invalid struct as the input

*Expected:* The result should be an error detailing the validation failure.

#### `TS-binding-default-validator-004` — ValidateStruct returns an error when input obj is an invalid struct

**P1** · unit · covers `RISK-binding-default-validator` · [`ValidateStruct` in binding/default_validator.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/default_validator.go)

*Steps:*

1. Create an instance of defaultValidator
2. Define an invalid struct according to validation rules
3. Call ValidateStruct with the invalid struct as the input

*Expected:* The result should be an error detailing the validation failure.

#### `TS-binding-default-validator-005` — ValidateStruct returns errors for all invalid elements in a slice of structs

**P1** · unit · covers `RISK-binding-default-validator` · [`ValidateStruct` in binding/default_validator.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/default_validator.go)

*Steps:*

1. Create an instance of defaultValidator
2. Define a slice containing both valid and invalid structs
3. Call ValidateStruct with the slice as the input

*Expected:* The result should be a SliceValidationError containing errors for all invalid elements.

#### `TS-binding-default-validator-006` — ValidateStruct returns nil when input obj is a valid slice/array of structs

**P1** · unit · covers `RISK-binding-default-validator` · [`ValidateStruct` in binding/default_validator.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/default_validator.go)

*Steps:*

1. Create an instance of defaultValidator
2. Define a valid slice/array of structs according to validation rules
3. Call ValidateStruct with the valid slice/array as the input

*Expected:* The result should be nil.

#### `TS-binding-default-validator-007` — ValidateStruct returns nil when input is a slice of pointers where all pointed structs are valid

**P1** · unit · covers `RISK-binding-default-validator` · [`ValidateStruct` in binding/default_validator.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/default_validator.go)

*Steps:*

1. Create an instance of defaultValidator
2. Define a slice of pointers to valid structs according to validation rules
3. Call ValidateStruct with the slice of pointers as the input

*Expected:* The result should be nil.

#### `TS-binding-default-validator-008` — ValidateStruct returns nil when input is an array of valid structs

**P1** · unit · covers `RISK-binding-default-validator` · [`ValidateStruct` in binding/default_validator.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/default_validator.go)

*Steps:*

1. Create an instance of defaultValidator
2. Define an array of valid structs according to validation rules
3. Call ValidateStruct with the array as the input

*Expected:* The result should be nil.

#### `TS-binding-default-validator-009` — ValidateStruct returns nil when input is an empty slice

**P1** · unit · covers `RISK-binding-default-validator` · [`ValidateStruct` in binding/default_validator.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/default_validator.go)

*Steps:*

1. Create an instance of defaultValidator
2. Call ValidateStruct with an empty slice as the input

*Expected:* The result should be nil.

#### `TS-binding-default-validator-010` — ValidateStruct returns nil when input is an empty array

**P1** · unit · covers `RISK-binding-default-validator` · [`ValidateStruct` in binding/default_validator.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/default_validator.go)

*Steps:*

1. Create an instance of defaultValidator
2. Call ValidateStruct with an empty array as the input

*Expected:* The result should be nil.

#### `TS-binding-default-validator-011` — ValidateStruct does not modify defaultValidator state when input obj is nil

**P1** · unit · covers `RISK-binding-default-validator` · [`ValidateStruct` in binding/default_validator.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/default_validator.go)

*Steps:*

1. Create an instance of defaultValidator
2. Call ValidateStruct with nil as the input
3. Check the state of the defaultValidator instance

*Expected:* The state of defaultValidator instance remains unchanged.

#### `TS-binding-default-validator-012` — ValidateStruct does not modify defaultValidator state when input obj is a valid struct

**P1** · unit · covers `RISK-binding-default-validator` · [`ValidateStruct` in binding/default_validator.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/default_validator.go)

*Steps:*

1. Create an instance of defaultValidator
2. Define a valid struct according to validation rules
3. Call ValidateStruct with the valid struct as the input
4. Check the state of the defaultValidator instance

*Expected:* The state of defaultValidator instance remains unchanged.

### binding/form

#### `TS-binding-form-001` — Bind method rejects an HTTP request with invalid form data

**P0** · negative · covers `RISK-binding-form` · [`Bind` in binding/form.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/form.go)

*Preconditions:*

- An HTTP request with invalid form data is prepared.

*Steps:*

1. Prepare an HTTP request with malformed form data.
2. Invoke the Bind method with the request and a new object.

*Expected:* The Bind method returns an error indicating form data parsing failure.

#### `TS-binding-form-002` — Bind method rejects an HTTP request with invalid multipart form data

**P0** · negative · covers `RISK-binding-form` · [`Bind` in binding/form.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/form.go)

*Preconditions:*

- An HTTP request with invalid multipart form data is prepared.

*Steps:*

1. Prepare an HTTP request with malformed multipart form data.
2. Invoke the Bind method with the request and a new object.

*Expected:* The Bind method returns an error indicating multipart form data parsing failure.

#### `TS-binding-form-003` — Bind method rejects when unable to map form data to object

**P0** · negative · covers `RISK-binding-form` · [`Bind` in binding/form.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/form.go)

*Preconditions:*

- An HTTP request with valid form data is prepared but mapping fails.

*Steps:*

1. Set up an object type that causes a mapping failure.
2. Invoke the Bind method with the request and the object.

*Expected:* The Bind method returns an error indicating form data mapping failure.

#### `TS-binding-form-004` — Bind method rejects when validation of object fails

**P0** · negative · covers `RISK-binding-form` · [`Bind` in binding/form.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/form.go)

*Preconditions:*

- An HTTP request with valid form data is prepared but validation fails.

*Steps:*

1. Prepare an HTTP request with valid form data.
2. Set up an object that fails validation.
3. Invoke the Bind method with the request and the object.

*Expected:* The Bind method returns an error indicating object validation failure.

#### `TS-binding-form-005` — Bind method successfully binds valid form data

**P0** · unit · covers `RISK-binding-form` · [`Bind` in binding/form.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/form.go)

*Preconditions:*

- An HTTP request with valid form data is prepared.

*Steps:*

1. Prepare an HTTP request with valid form data.
2. Invoke the Bind method with the request and a new object.
3. Check the object for expected values after binding.

*Expected:* The Bind method successfully binds the form data to the object without errors.

### binding/form_mapping

#### `TS-binding-form-mapping-001` — MapFormWithTag fails on invalid pointer reference

**P0** · negative · covers `RISK-binding-form-mapping` · [`MapFormWithTag` in binding/form_mapping.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/form_mapping.go)

*Preconditions:*

- Create a function with an invalid pointer type.

*Steps:*

1. Call MapFormWithTag(nil, form, tag) where form = map[string][]string and tag = 'form'.
2. Check if the method returns an error stating 'invalid pointer reference'.

*Expected:* The method returns an error stating 'invalid pointer reference'.

#### `TS-binding-form-mapping-002` — MapFormWithTag fails on unsupported type for mapping

**P0** · negative · covers `RISK-binding-form-mapping` · [`MapFormWithTag` in binding/form_mapping.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/form_mapping.go)

*Preconditions:*

- Create a form map and a struct with unsupported field type.

*Steps:*

1. Call MapFormWithTag(pointerToStruct, form, tag) where pointerToStruct is of unsupported type.
2. Check if the method returns an error stating 'unsupported type for mapping'.

*Expected:* The method returns an error stating 'unsupported type for mapping'.

#### `TS-binding-form-mapping-003` — MapFormWithTag fails on invalid form data format

**P0** · negative · covers `RISK-binding-form-mapping` · [`MapFormWithTag` in binding/form_mapping.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/form_mapping.go)

*Preconditions:*

- Create a form map with invalid data format.

*Steps:*

1. Call MapFormWithTag(pointerToStruct, invalidForm, tag) where invalidForm is not formatted properly.
2. Check if the method returns an error due to an invalid data format.

*Expected:* The method returns an error indicating an invalid form data format.

#### `TS-binding-form-mapping-004` — MapFormWithTag fails to parse value

**P0** · negative · covers `RISK-binding-form-mapping` · [`MapFormWithTag` in binding/form_mapping.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/form_mapping.go)

*Preconditions:*

- Create a form map that includes a value that cannot be parsed.

*Steps:*

1. Call MapFormWithTag(pointerToStruct, formWithUnparsableValue, tag).
2. Check if the method returns an error indicating it failed to parse the value.

*Expected:* The method returns an error indicating it failed to parse the value.

#### `TS-binding-form-mapping-005` — MapFormWithTag fails on incompatible types for mapping

**P0** · negative · covers `RISK-binding-form-mapping` · [`MapFormWithTag` in binding/form_mapping.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/form_mapping.go)

*Preconditions:*

- Create a form map with data types that do not match the struct fields.

*Steps:*

1. Call MapFormWithTag(pointerToStruct, incompatibleForm, tag).
2. Check if the method returns an error due to incompatible types.

*Expected:* The method returns an error indicating incompatible types for mapping.

### binding/header

#### `TS-binding-header-001` — Bind returns an error if mapHeader fails

**P1** · negative · covers `RISK-binding-header` · [`Bind` in binding/header.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/header.go)

*Preconditions:*

- A HTTP request with headers is prepared
- An object is defined that cannot be mapped to the headers

*Steps:*

1. Call the Bind method on headerBinding with the prepared request and the defined object

*Expected:* The Bind method returns an error indicating the mapping failure.

#### `TS-binding-header-002` — Bind returns an error if validate fails

**P1** · negative · covers `RISK-binding-header` · [`Bind` in binding/header.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/header.go)

*Preconditions:*

- A HTTP request with valid headers is prepared
- An object is defined that fails validation

*Steps:*

1. Call the Bind method on headerBinding with the prepared request and the defined object

*Expected:* The Bind method returns an error indicating validation failure.

#### `TS-binding-header-003` — Bind successfully maps headers to an object

**P1** · unit · covers `RISK-binding-header` · [`Bind` in binding/header.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/header.go)

*Preconditions:*

- A HTTP request with valid headers is prepared
- An object is defined that can be mapped to the headers

*Steps:*

1. Call the Bind method on headerBinding with the prepared request and the defined object

*Expected:* The Bind method returns nil indicating success and the object is correctly populated with header values.

#### `TS-binding-header-004` — Bind does not modify obj when an error occurs

**P1** · negative · covers `RISK-binding-header` · [`Bind` in binding/header.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/header.go)

*Steps:*

1. Create a mock HTTP request with headers that cause mapHeader to return an error
2. Create an instance of the object to bind to
3. Store the initial state of the object
4. Call the Bind method with the request and object
5. Verify the object has not been modified

*Expected:* The object remains in its initial state.

### binding/json

#### `TS-binding-json-001` — Bind returns error for nil request

**P1** · negative · covers `RISK-binding-json` · [`Bind` in binding/json.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/json.go)

*Steps:*

1. Call Bind with nil request and a valid object

*Expected:* The function returns 'invalid request' error.

#### `TS-binding-json-002` — Bind returns error for request with nil body

**P1** · negative · covers `RISK-binding-json` · [`Bind` in binding/json.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/json.go)

*Steps:*

1. Create an HTTP request with nil body
2. Call Bind with the request and a valid object

*Expected:* The function returns 'invalid request' error.

#### `TS-binding-json-003` — Bind returns JSON decoding error for invalid JSON

**P1** · negative · covers `RISK-binding-json` · [`BindBody` in binding/json.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/json.go)

*Steps:*

1. Call BindBody with an invalid JSON byte array and a valid object

*Expected:* The function returns a JSON decoding error.

#### `TS-binding-json-004` — Bind fails validation for invalid object data

**P1** · negative · covers `RISK-binding-json` · [`BindBody` in binding/json.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/json.go)

*Steps:*

1. Call BindBody with valid JSON but data that does not satisfy validation rules for the object

*Expected:* The function returns a validation error.

#### `TS-binding-json-005` — Bind successfully decodes valid JSON to object

**P1** · unit · covers `RISK-binding-json` · [`Bind` in binding/json.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/json.go)

*Steps:*

1. Create a valid HTTP request with a valid JSON body
2. Call Bind with the request and a valid object

*Expected:* The object is correctly populated with the decoded JSON data.

### binding/msgpack

#### `TS-binding-msgpack-001` — Bind fails with invalid MsgPack format

**P1** · negative · covers `RISK-binding-msgpack` · [`Bind` in binding/msgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/msgpack.go)

*Preconditions:*

- HTTP request with invalid MsgPack body is sent

*Steps:*

1. Send HTTP request with invalid MsgPack body to the Bind function
2. Capture the error returned from Bind function

*Expected:* Error indicating invalid MsgPack format is returned.

#### `TS-binding-msgpack-002` — Bind fails when validation of decoded object fails

**P1** · negative · covers `RISK-binding-msgpack` · [`Bind` in binding/msgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/msgpack.go)

*Preconditions:*

- HTTP request with valid MsgPack format is sent
- The validation function fails for the decoded object

*Steps:*

1. Send HTTP request with valid MsgPack body to the Bind function
2. Capture the error returned from Bind function

*Expected:* Error indicating validation failure is returned.

#### `TS-binding-msgpack-003` — Bind fails when reading request body fails

**P1** · negative · covers `RISK-binding-msgpack` · [`Bind` in binding/msgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/msgpack.go)

*Preconditions:*

- Simulate an error reading the HTTP request body

*Steps:*

1. Send HTTP request to the Bind function while simulating an error in request body reading
2. Capture the error returned from Bind function

*Expected:* Error indicating failure in reading request body is returned.

#### `TS-binding-msgpack-004` — BindBody fails with invalid MsgPack format

**P1** · negative · covers `RISK-binding-msgpack` · [`BindBody` in binding/msgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/msgpack.go)

*Preconditions:*

- Body containing invalid MsgPack data is provided

*Steps:*

1. Call BindBody with invalid MsgPack byte array
2. Capture the error returned from BindBody

*Expected:* Error indicating invalid MsgPack format is returned.

#### `TS-binding-msgpack-005` — BindBody fails when validation of decoded object fails

**P1** · negative · covers `RISK-binding-msgpack` · [`BindBody` in binding/msgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/msgpack.go)

*Preconditions:*

- A valid MsgPack byte array is provided
- The validation function fails for the decoded object

*Steps:*

1. Call BindBody with valid MsgPack byte array that fails validation
2. Capture the error returned from BindBody

*Expected:* Error indicating validation failure is returned.

#### `TS-binding-msgpack-006` — BindBody fails when reading from the byte slice results in an error

**P1** · negative · covers `RISK-binding-msgpack` · [`msgpackBinding.BindBody` in binding/msgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/msgpack.go)

*Preconditions:*

- A byte slice simulating an error when creating a reader is established.

*Steps:*

1. Mock the byte slice to simulate an error when creating the NewReader.
2. Call msgpackBinding.BindBody with the simulated error byte slice and a target object.

*Expected:* The method returns an error indicating failure due to reading issues.

### binding/multipart_form_mapping

#### `TS-binding-multipart-form-mapping-001` — TrySet fails when no files are found in multipart request

**P1** · negative · covers `RISK-binding-multipart-form-mapping` · [`TrySet` in binding/multipart_form_mapping.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/multipart_form_mapping.go)

*Preconditions:*

- A multipart request with an empty MultipartForm

*Steps:*

1. Set up an instance of multipartRequest without any File in MultipartForm.
2. Call TrySet on this instance with appropriate reflect.Value and reflect.StructField that corresponds to a file type.

*Expected:* TrySet returns an error indicating no files were found.

#### `TS-binding-multipart-form-mapping-002` — TrySet fails with invalid type for multipart.FileHeader

**P1** · negative · covers `RISK-binding-multipart-form-mapping` · [`TrySet` in binding/multipart_form_mapping.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/multipart_form_mapping.go)

*Preconditions:*

- A multipart request with a non-file type in reflection

*Steps:*

1. Set up an instance of multipartRequest with File in MultipartForm but cast to an unsupported type.
2. Call TrySet with the instance.

*Expected:* TrySet returns an error of type ErrMultiFileHeader.

#### `TS-binding-multipart-form-mapping-003` — TrySet fails with invalid length of multipart.FileHeader array

**P1** · negative · covers `RISK-binding-multipart-form-mapping` · [`TrySet` in binding/multipart_form_mapping.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/multipart_form_mapping.go)

*Preconditions:*

- A multipart request with an array size mismatch

*Steps:*

1. Set up an instance of multipartRequest with an array of multipart.FileHeader of different length than expected.
2. Call TrySet with this instance.

*Expected:* TrySet returns an error of type ErrMultiFileHeaderLenInvalid.

#### `TS-binding-multipart-form-mapping-004` — TrySet succeeds when valid multipart.FileHeader is provided

**P1** · unit · covers `RISK-binding-multipart-form-mapping` · [`TrySet` in binding/multipart_form_mapping.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/multipart_form_mapping.go)

*Preconditions:*

- A multipart request with valid File in MultipartForm

*Steps:*

1. Set up an instance of multipartRequest with at least one valid multipart.FileHeader in MultipartForm.
2. Call TrySet with an appropriate reflect.Value and reflect.StructField.

*Expected:* TrySet sets the value correctly without errors.

### binding/plain

#### `TS-binding-plain-001` — Bind rejects a nil object passed for binding

**P1** · negative · covers `RISK-binding-plain` · [`Bind` in binding/plain.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/plain.go)

*Preconditions:*

- Create an HTTP request with a valid body

*Steps:*

1. Create a plainBinding instance
2. Call Bind method with the HTTP request and a nil object

*Expected:* The method returns nil, indicating no mutation of the object.

#### `TS-binding-plain-002` — Bind rejects unsupported object type

**P1** · negative · covers `RISK-binding-plain` · [`Bind` in binding/plain.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/plain.go)

*Preconditions:*

- Create an HTTP request with a valid body

*Steps:*

1. Create a plainBinding instance
2. Create an object of an unsupported type (e.g., int)
3. Call Bind method with the HTTP request and the unsupported object

*Expected:* The method returns an error indicating unknown type.

#### `TS-binding-plain-003` — Bind returns error when reading request body fails

**P1** · negative · covers `RISK-binding-plain` · [`Bind` in binding/plain.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/plain.go)

*Preconditions:*

- Create an HTTP request with an invalid body that cannot be read

*Steps:*

1. Create a plainBinding instance
2. Call Bind method with an invalid HTTP request that simulates a body read error

*Expected:* The method returns an error indicating a failure to read the request body.

#### `TS-binding-plain-004` — Bind mutates a string object with the request body

**P1** · unit · covers `RISK-binding-plain` · [`Bind` in binding/plain.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/plain.go)

*Preconditions:*

- Create an HTTP request with a valid body that can be converted to a string

*Steps:*

1. Create a plainBinding instance
2. Create a string variable to bind to
3. Call Bind method with the HTTP request and the string variable

*Expected:* The string variable contains the contents of the request body.

#### `TS-binding-plain-005` — Bind mutates a byte slice object with the request body

**P1** · unit · covers `RISK-binding-plain` · [`Bind` in binding/plain.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/plain.go)

*Preconditions:*

- Create an HTTP request with a valid body that can be read as bytes

*Steps:*

1. Create a plainBinding instance
2. Create a byte slice variable to bind to
3. Call Bind method with the HTTP request and the byte slice variable

*Expected:* The byte slice variable contains the contents of the request body.

### binding/query

#### `TS-binding-query-001` — Bind fails when mapping form data results in an error

**P1** · negative · covers `RISK-binding-query` · [`Bind` in binding/query.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/query.go)

*Preconditions:*

- An HTTP request contains query parameters that cannot be mapped to the given object.

*Steps:*

1. Create an HTTP request with invalid query parameters
2. Create an object of the expected type for binding
3. Call the Bind method with the request and the object

*Expected:* The Bind method returns an error indicating the failure in mapping.

#### `TS-binding-query-002` — Bind fails when validation of the object fails

**P1** · negative · covers `RISK-binding-query` · [`Bind` in binding/query.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/query.go)

*Preconditions:*

- An HTTP request contains valid query parameters but the resulting object fails validation.

*Steps:*

1. Create an HTTP request with query parameters that map correctly to the object
2. Create the object with values that do not pass validation rules
3. Call the Bind method with the request and the object

*Expected:* The Bind method returns an error indicating validation failure.

#### `TS-binding-query-003` — Bind succeeds when query parameters are valid and mapped correctly

**P1** · unit · covers `RISK-binding-query` · [`Bind` in binding/query.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/query.go)

*Preconditions:*

- An HTTP request contains valid query parameters that can be mapped to the given object.

*Steps:*

1. Create an HTTP request with valid query parameters
2. Create an object of the expected type for binding
3. Call the Bind method with the request and the object

*Expected:* The Bind method returns no error and the object is populated as per query parameters.

### binding/toml

#### `TS-binding-toml-001` — Bind fails with invalid TOML input from HTTP request

**P1** · negative · covers `RISK-binding-toml` · [`Bind` in binding/toml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/toml.go)

*Preconditions:*

- Server is running and capable of receiving HTTP requests

*Steps:*

1. Send an HTTP POST request to the server with invalid TOML data in the body

*Expected:* The server responds with a 400 Bad Request status code.

#### `TS-binding-toml-002` — BindBody fails with invalid TOML input from byte array

**P1** · negative · covers `RISK-binding-toml` · [`BindBody` in binding/toml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/toml.go)

*Steps:*

1. Call BindBody with a byte array containing invalid TOML data
2. Expect it to return an error

*Expected:* The error returned is non-nil, indicating a decoding failure.

#### `TS-binding-toml-003` — Bind fails when net/http.Request has no body

**P1** · negative · covers `RISK-binding-toml` · [`Bind` in binding/toml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/toml.go)

*Steps:*

1. Create an HTTP request with no body
2. Call the Bind method with this request

*Expected:* The method returns an error indicating that the body is empty.

#### `TS-binding-toml-004` — BindBody fails with empty byte array

**P1** · negative · covers `RISK-binding-toml` · [`BindBody` in binding/toml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/toml.go)

*Steps:*

1. Call BindBody with an empty byte array
2. Expect it to return an error

*Expected:* The error returned is non-nil, indicating that the input is invalid.

#### `TS-binding-toml-005` — Bind fails when TOML validation fails

**P1** · negative · covers `RISK-binding-toml` · [`Bind` in binding/toml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/toml.go)

*Steps:*

1. Send an HTTP POST request with a valid but non-compliant TOML object
2. Expect validation to fail and return an error

*Expected:* The server responds with a 422 Unprocessable Entity status code.

#### `TS-binding-toml-006` — BindBody successfully decodes valid TOML input from byte array

**P1** · unit · covers `RISK-binding-toml` · [`BindBody` in binding/toml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/toml.go)

*Preconditions:*

- A byte array with valid TOML data

*Steps:*

1. Prepare a byte array containing valid TOML data
2. Invoke the BindBody method with the byte array and a destination object
3. Check that the destination object has been populated correctly

*Expected:* The destination object is populated with the correct TOML values.

### binding/xml

#### `TS-binding-xml-001` — Attempt to Bind with invalid XML format

**P1** · negative · covers `RISK-binding-xml` · [`xmlBinding.Bind` in binding/xml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/xml.go)

*Steps:*

1. Create an HTTP request with an invalid XML body
2. Instantiate a new xmlBinding object
3. Call the Bind method with the HTTP request and a destination object

*Expected:* The Bind method returns an error indicating invalid XML format.

#### `TS-binding-xml-002` — Attempt to Bind with empty request body

**P1** · negative · covers `RISK-binding-xml` · [`xmlBinding.Bind` in binding/xml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/xml.go)

*Steps:*

1. Create an HTTP request with an empty body
2. Instantiate a new xmlBinding object
3. Call the Bind method with the HTTP request and a destination object

*Expected:* The Bind method returns an error indicating that the request body is empty.

#### `TS-binding-xml-003` — Attempt to BindBody with invalid XML format

**P1** · negative · covers `RISK-binding-xml` · [`xmlBinding.BindBody` in binding/xml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/xml.go)

*Steps:*

1. Create a byte array containing invalid XML
2. Instantiate a new xmlBinding object
3. Call the BindBody method with the invalid XML byte array and a destination object

*Expected:* The BindBody method returns an error indicating invalid XML format.

#### `TS-binding-xml-004` — Attempt to BindBody with empty body

**P1** · negative · covers `RISK-binding-xml` · [`xmlBinding.BindBody` in binding/xml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/xml.go)

*Steps:*

1. Create an empty byte array
2. Instantiate a new xmlBinding object
3. Call the BindBody method with the empty byte array and a destination object

*Expected:* The BindBody method returns an error indicating that the body is empty.

#### `TS-binding-xml-005` — Bind object fails validation after decoding

**P1** · negative · covers `RISK-binding-xml` · [`xmlBinding.Bind` in binding/xml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/xml.go)

*Steps:*

1. Create an HTTP request with a valid XML body
2. Instantiate a new xmlBinding object
3. Stub the validate function to always return an error
4. Call the Bind method with the HTTP request and a destination object

*Expected:* The Bind method returns an error indicating that the object failed validation.

#### `TS-binding-xml-006` — BindBody object fails validation after decoding

**P1** · negative · covers `RISK-binding-xml` · [`xmlBinding.BindBody` in binding/xml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/xml.go)

*Steps:*

1. Create a byte array containing valid XML
2. Instantiate a new xmlBinding object
3. Stub the validate function to always return an error
4. Call the BindBody method with the byte array and a destination object

*Expected:* The BindBody method returns an error indicating that the object failed validation.

#### `TS-binding-xml-007` — BindBody mutates the object after successful binding

**P1** · unit · covers `RISK-binding-xml` · [`BindBody` in binding/xml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/xml.go)

*Steps:*

1. Call the BindBody function with valid XML byte array
2. Create an object with predefined values
3. Call the BindBody method with the byte array and the object

*Expected:* The object reflects the values parsed from the XML byte array.

### binding/yaml

#### `TS-binding-yaml-001` — Bind handles YAML decoding failure gracefully

**P1** · negative · covers `RISK-binding-yaml` · [`yamlBinding.Bind` in binding/yaml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/yaml.go)

*Steps:*

1. Create an HTTP request with an invalid YAML body
2. Create a dummy struct to bind the YAML to
3. Call yamlBinding.Bind with the request and the struct

*Expected:* The method returns an error indicating that the YAML decoding failed.

#### `TS-binding-yaml-002` — BindBody handles YAML decoding failure gracefully

**P1** · negative · covers `RISK-binding-yaml` · [`yamlBinding.BindBody` in binding/yaml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/yaml.go)

*Steps:*

1. Prepare a byte slice with invalid YAML content
2. Create a dummy struct to bind the YAML to
3. Call yamlBinding.BindBody with the byte slice and the struct

*Expected:* The method returns an error indicating that the YAML decoding failed.

#### `TS-binding-yaml-003` — Bind fails if the bound object does not pass validation

**P1** · negative · covers `RISK-binding-yaml` · [`yamlBinding.Bind` in binding/yaml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/yaml.go)

*Steps:*

1. Create an HTTP request with a valid YAML body
2. Create a dummy struct that fails validation
3. Call yamlBinding.Bind with the request and the struct

*Expected:* The method returns an error indicating that the validation of the bound object failed.

#### `TS-binding-yaml-004` — BindBody fails if the bound object does not pass validation

**P1** · negative · covers `RISK-binding-yaml` · [`yamlBinding.BindBody` in binding/yaml.go](https://github.com/gin-gonic/gin/blob/HEAD/binding/yaml.go)

*Steps:*

1. Prepare a byte slice with valid YAML content
2. Create a dummy struct that fails validation
3. Call yamlBinding.BindBody with the byte slice and the struct

*Expected:* The method returns an error indicating that the validation of the bound object failed.

### debug

#### `TS-debug-001` — Verify debug output when debug mode is enabled

**P1** · unit · covers `RISK-debug` · [`debugPrint` in debug.go](https://github.com/gin-gonic/gin/blob/HEAD/debug.go)

*Preconditions:*

- Set debug mode to true

*Steps:*

1. Call debugPrint with a formatted string and values
2. Check the output stream for the formatted string

*Expected:* The output stream contains the correctly formatted debug message.

#### `TS-debug-002` — Check error handling when writing to debug output stream fails

**P1** · unit · covers `RISK-debug` · [`debugPrint` in debug.go](https://github.com/gin-gonic/gin/blob/HEAD/debug.go)

*Preconditions:*

- Set debug mode to true
- Simulate an error on the output stream

*Steps:*

1. Call debugPrint with a formatted string and values
2. Observe the program behavior

*Expected:* No runtime crash occurs, and error handling is triggered without crashing.

#### `TS-debug-003` — Verify handling of runtime errors during debug print

**P1** · unit · covers `RISK-debug` · [`debugPrint` in debug.go](https://github.com/gin-gonic/gin/blob/HEAD/debug.go)

*Preconditions:*

- Set debug mode to true

*Steps:*

1. Corrupt the formatting string by introducing a syntax error
2. Call debugPrint with the corrupted formatting string

*Expected:* The system handles the error gracefully without crashing and continues execution.

#### `TS-debug-004` — Ensure warning message is logged when using old Go version in debug mode

**P1** · unit · covers `RISK-debug` · [`debugPrintWARNINGDefault` in debug.go](https://github.com/gin-gonic/gin/blob/HEAD/debug.go)

*Preconditions:*

- Set debug mode to true
- Set Go version below 1.25

*Steps:*

1. Call debugPrintWARNINGDefault

*Expected:* The output stream contains a warning message about the Go version.

#### `TS-debug-005` — Verify debugPrintError function with a valid error

**P1** · unit · covers `RISK-debug` · [`debugPrintError` in debug.go](https://github.com/gin-gonic/gin/blob/HEAD/debug.go)

*Preconditions:*

- Set debug mode to true

*Steps:*

1. Call debugPrintError with a non-nil error

*Expected:* The output stream contains the error message formatted correctly.

#### `TS-debug-006` — Ensure debugPrintError does not log when there is no error

**P1** · unit · covers `RISK-debug` · [`debugPrintError` in debug.go](https://github.com/gin-gonic/gin/blob/HEAD/debug.go)

*Preconditions:*

- Set debug mode to true

*Steps:*

1. Call debugPrintError with a nil error
2. Observe the output stream

*Expected:* No error message is logged in the output stream.

#### `TS-debug-007` — Ensure that debugPrintLoadTemplate outputs loaded templates in debug mode

**P1** · unit · covers `RISK-debug` · [`debugPrintLoadTemplate` in debug.go](https://github.com/gin-gonic/gin/blob/HEAD/debug.go)

*Preconditions:*

- Set debug mode to true
- Load some HTML templates

*Steps:*

1. Call debugPrintLoadTemplate with the loaded templates
2. Check the output stream

*Expected:* The output stream contains names of the loaded HTML templates.

#### `TS-debug-008` — Check that templates are not logged when debug mode is off

**P1** · unit · covers `RISK-debug` · [`debugPrintLoadTemplate` in debug.go](https://github.com/gin-gonic/gin/blob/HEAD/debug.go)

*Preconditions:*

- Set debug mode to false
- Load some HTML templates

*Steps:*

1. Call debugPrintLoadTemplate with the loaded templates
2. Observe the output stream

*Expected:* No output is logged in the output stream.

#### `TS-debug-009` — Validate debug mode is off when gin mode is release

**P1** · unit · covers `RISK-debug` · [`IsDebugging` in debug.go](https://github.com/gin-gonic/gin/blob/HEAD/debug.go)

*Steps:*

1. Set gin mode to release
2. Call IsDebugging()
3. Verify the response

*Expected:* IsDebugging() should return false.

### errors

#### `TS-errors-001` — Instantiate Error with nil error

**P1** · negative · covers `RISK-errors` · [`Error` in errors.go](https://github.com/gin-gonic/gin/blob/HEAD/errors.go)

*Steps:*

1. Instantiate an Error object with a nil error value (e.g., `Error{Err: nil}`).

*Expected:* The Error object is created without panic.

#### `TS-errors-002` — Attempt JSON serialization with nil Error

**P1** · negative · covers `RISK-errors` · [`Error.JSON` in errors.go](https://github.com/gin-gonic/gin/blob/HEAD/errors.go)

*Steps:*

1. Create an instance of Error with a nil error value.
2. Call the JSON() method on the Error instance.

*Expected:* The method call returns a JSON representation without panic.

#### `TS-errors-003` — Try to MarshalJSON on Error with nil error

**P1** · negative · covers `RISK-errors` · [`Error.MarshalJSON` in errors.go](https://github.com/gin-gonic/gin/blob/HEAD/errors.go)

*Steps:*

1. Create an instance of Error with a nil error value.
2. Call the MarshalJSON() method on the Error instance.

*Expected:* The method call returns an error indicating failure to marshal.

#### `TS-errors-004` — Call IsType with an unsupported ErrorType

**P1** · negative · covers `RISK-errors` · [`Error.IsType` in errors.go](https://github.com/gin-gonic/gin/blob/HEAD/errors.go)

*Steps:*

1. Create an instance of Error with a specific ErrorType.
2. Call the IsType() method with a different ErrorType.

*Expected:* The method returns false without panic.

#### `TS-errors-005` — Call String method on empty errorMsgs

**P1** · boundary · covers `RISK-errors` · [`errorMsgs.String` in errors.go](https://github.com/gin-gonic/gin/blob/HEAD/errors.go)

*Steps:*

1. Create an instance of errorMsgs with no elements.
2. Call the String() method on the empty errorMsgs instance.

*Expected:* The method returns an empty string.

#### `TS-errors-006` — Add errors to errorMsgs and call Errors method

**P1** · integration · covers `RISK-errors` · [`errorMsgs.Errors` in errors.go](https://github.com/gin-gonic/gin/blob/HEAD/errors.go)

*Steps:*

1. Instantiate an errorMsgs slice.
2. Add multiple Error instances to the slice.
3. Call the Errors() method.

*Expected:* The method returns a slice containing the error messages from the added Error instances.

#### `TS-errors-007` — Check String method on errorMsgs with a single error

**P1** · unit · covers `RISK-errors` · [`String` in errors.go](https://github.com/gin-gonic/gin/blob/HEAD/errors.go)

*Steps:*

1. Create an errorMsgs instance with one Error instance.
2. Call the String method.

*Expected:* The result should contain a formatted string representation of the single error.

#### `TS-errors-008` — Check String method on empty errorMsgs

**P1** · negative · covers `RISK-errors` · [`String` in errors.go](https://github.com/gin-gonic/gin/blob/HEAD/errors.go)

*Steps:*

1. Create an empty errorMsgs instance.
2. Call the String method.

*Expected:* The result should be an empty string.

### fs

#### `TS-fs-001` — Open returns error when file does not exist

**P1** · negative · covers `RISK-fs` · [`OnlyFilesFS.Open` in fs.go](https://github.com/gin-gonic/gin/blob/HEAD/fs.go)

*Steps:*

1. Create an instance of OnlyFilesFS with a mock FileSystem that returns an error when Open is called.
2. Attempt to open a non-existent file using the Open method.

*Expected:* Receive an error indicating that the file does not exist.

#### `TS-fs-002` — Open returns error when permission is denied

**P1** · negative · covers `RISK-fs` · [`OnlyFilesFS.Open` in fs.go](https://github.com/gin-gonic/gin/blob/HEAD/fs.go)

*Steps:*

1. Create an instance of OnlyFilesFS with a mock FileSystem that returns a permission denied error when Open is called.
2. Attempt to open a file that the mock reports as permission denied.

*Expected:* Receive an error indicating permission denied.

#### `TS-fs-003` — Open successfully returns a file when existing file is accessed

**P1** · unit · covers `RISK-fs` · [`OnlyFilesFS.Open` in fs.go](https://github.com/gin-gonic/gin/blob/HEAD/fs.go)

*Steps:*

1. Create an instance of OnlyFilesFS with a mock FileSystem that returns a valid file for Open when called.
2. Attempt to open an existing file using the Open method.

*Expected:* Receive a non-nil http.File when an existing file is opened.

#### `TS-fs-004` — Readdir always returns nil when called

**P1** · unit · covers `RISK-fs` · [`neutralizedReaddirFile.Readdir` in fs.go](https://github.com/gin-gonic/gin/blob/HEAD/fs.go)

*Steps:*

1. Create an instance of neutralizedReaddirFile wrapping a valid http.File.
2. Call the Readdir method with any integer argument.

*Expected:* Receive a nil slice and nil error when Readdir is called.

### gin

#### `TS-gin-001` — Handle invalid IP address in trusted proxies

**P0** · negative · covers `RISK-gin` · [`SetTrustedProxies` in gin.go](https://github.com/gin-gonic/gin/blob/HEAD/gin.go)

*Preconditions:*

- Start the gin server

*Steps:*

1. Attempt to set trusted proxies with an invalid IP 'invalid-ip'

*Expected:* An error is returned indicating the invalid IP address.

#### `TS-gin-002` — Handle error during HTTP server creation

**P0** · negative · covers `RISK-gin` · [`Run` in gin.go](https://github.com/gin-gonic/gin/blob/HEAD/gin.go)

*Preconditions:*

- Configure server with invalid options

*Steps:*

1. Attempt to run the gin server with an invalid address

*Expected:* An error is returned indicating the server creation failure.

#### `TS-gin-003` — Handle failure to write response

**P0** · negative · covers `RISK-gin` · [`ServeHTTP` in gin.go](https://github.com/gin-gonic/gin/blob/HEAD/gin.go)

*Preconditions:*

- Start the gin server and prepare an invalid HTTP writer

*Steps:*

1. Send a request to the server that simulates a failure in writing the response

*Expected:* An appropriate error is logged indicating the failure in writing the response.

#### `TS-gin-004` — Handle error during request redirect

**P0** · negative · covers `RISK-gin` · [`redirectRequest` in gin.go](https://github.com/gin-gonic/gin/blob/HEAD/gin.go)

*Preconditions:*

- Start the gin server

*Steps:*

1. Simulate a request that triggers a redirect failure (e.g., invalid URL)

*Expected:* The server logs an error message indicating a failure to redirect.

#### `TS-gin-005` — Handle failure to parse CIDR

**P0** · negative · covers `RISK-gin` · [`prepareTrustedCIDRs` in gin.go](https://github.com/gin-gonic/gin/blob/HEAD/gin.go)

*Preconditions:*

- Start the gin server

*Steps:*

1. Set trusted proxies with an invalid CIDR 'invalid-cidr'

*Expected:* An error is returned indicating the failure to parse the CIDR.

### ginS/gins

#### `TS-gins-gins-001` — Run fails when the address is unavailable

**P1** · negative · covers `RISK-gins-gins` · [`Run` in ginS/gins.go](https://github.com/gin-gonic/gin/blob/HEAD/ginS/gins.go)

*Steps:*

1. Attempt to start the HTTP server using an address that is currently in use, e.g., localhost:8080 if another server is running on that port.

*Expected:* The server fails to start and returns an error indicating the address is already in use.

#### `TS-gins-gins-002` — RunTLS fails when the certificate files are invalid

**P1** · negative · covers `RISK-gins-gins` · [`RunTLS` in ginS/gins.go](https://github.com/gin-gonic/gin/blob/HEAD/ginS/gins.go)

*Steps:*

1. Attempt to start the HTTPS server with invalid certificate and key files.

*Expected:* The server fails to start and returns an error related to the certificate validation.

#### `TS-gins-gins-003` — RunUnix fails when the socket file is invalid

**P1** · negative · covers `RISK-gins-gins` · [`RunUnix` in ginS/gins.go](https://github.com/gin-gonic/gin/blob/HEAD/ginS/gins.go)

*Steps:*

1. Attempt to start the HTTP server using an invalid Unix socket file path.

*Expected:* The server fails to start and returns an error indicating the Unix socket could not be accessed.

#### `TS-gins-gins-004` — Handle returns error when passed invalid route configuration

**P1** · negative · covers `RISK-gins-gins` · [`Handle` in ginS/gins.go](https://github.com/gin-gonic/gin/blob/HEAD/ginS/gins.go)

*Steps:*

1. Attempt to add a route with an invalid HTTP method and relative path.

*Expected:* The method returns an error indicating the HTTP method is invalid.

#### `TS-gins-gins-005` — LoadHTMLGlob fails when the file pattern does not match any files

**P1** · negative · covers `RISK-gins-gins` · [`LoadHTMLGlob` in ginS/gins.go](https://github.com/gin-gonic/gin/blob/HEAD/ginS/gins.go)

*Steps:*

1. Call LoadHTMLGlob with a non-existent pattern.

*Expected:* An error is returned indicating no matching files were found for the provided pattern.

#### `TS-gins-gins-006` — Group creates a new router group with specified path

**P1** · unit · covers `RISK-gins-gins` · [`Group` in ginS/gins.go](https://github.com/gin-gonic/gin/blob/HEAD/ginS/gins.go)

*Steps:*

1. Create a new route group by calling Group with a specific path.
2. Verify that the returned group is not nil.

*Expected:* The returned RouterGroup is successfully created and is not nil.

#### `TS-gins-gins-007` — NoMethod registers a handler but returns 405 for unsupported HTTP method

**P1** · negative · covers `RISK-gins-gins` · [`NoMethod` in ginS/gins.go](https://github.com/gin-gonic/gin/blob/HEAD/ginS/gins.go)

*Steps:*

1. Register a NoMethod handler and then make a request with an unsupported method to that route.

*Expected:* The response status code is 405.

### json/go_json

#### `TS-json-go-json-001` — Marshal fails with invalid input

**P0** · negative · covers `RISK-json-go-json` · [`gojsonApi.Marshal` in codec/json/go_json.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/go_json.go)

*Steps:*

1. Call Marshal with a reference to a channel or a function (invalid input).

*Expected:* An error is returned indicating that the input type cannot be marshaled.

#### `TS-json-go-json-002` — Unmarshal fails with invalid JSON data

**P0** · negative · covers `RISK-json-go-json` · [`gojsonApi.Unmarshal` in codec/json/go_json.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/go_json.go)

*Steps:*

1. Call Unmarshal with malformed JSON data (e.g., '{invalid_json}')

*Expected:* An error is returned indicating that the JSON cannot be unmarshaled.

#### `TS-json-go-json-003` — MarshalIndent fails with invalid input

**P0** · negative · covers `RISK-json-go-json` · [`gojsonApi.MarshalIndent` in codec/json/go_json.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/go_json.go)

*Steps:*

1. Call MarshalIndent with invalid input data (e.g., a channel).

*Expected:* An error is returned indicating that the input type cannot be marshaled with indentation.

#### `TS-json-go-json-004` — NewEncoder writes to io.Writer successfully

**P0** · unit · covers `RISK-json-go-json` · [`gojsonApi.NewEncoder` in codec/json/go_json.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/go_json.go)

*Steps:*

1. Create a bytes.Buffer as io.Writer.
2. Create a new encoder using NewEncoder and pass the bytes.Buffer.
3. Call the encoder's Encode method with valid input.

*Expected:* The byte buffer contains correctly encoded JSON data.

#### `TS-json-go-json-005` — NewDecoder reads from io.Reader successfully

**P0** · unit · covers `RISK-json-go-json` · [`gojsonApi.NewDecoder` in codec/json/go_json.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/go_json.go)

*Steps:*

1. Create a bytes.Buffer with valid JSON data as io.Reader.
2. Create a new decoder using NewDecoder and pass the bytes.Buffer.
3. Call the decoder's Decode method and pass a pointer to an expected struct.

*Expected:* The expected struct is populated with data from the JSON.

### json/json

#### `TS-json-json-001` — Marshal returns an error for unsupported data type

**P0** · negative · covers `RISK-json-json` · [`Marshal` in codec/json/json.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/json.go)

*Steps:*

1. Call Marshal with a channel type as input
2. Check the returned error

*Expected:* An error is returned for unsupported data type

#### `TS-json-json-002` — Unmarshal returns an error for invalid JSON

**P0** · negative · covers `RISK-json-json` · [`Unmarshal` in codec/json/json.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/json.go)

*Steps:*

1. Call Unmarshal with an invalid JSON string
2. Check the returned error

*Expected:* An error is returned indicating invalid JSON

#### `TS-json-json-003` — MarshalIndent returns an error for unsupported data type

**P0** · negative · covers `RISK-json-json` · [`MarshalIndent` in codec/json/json.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/json.go)

*Steps:*

1. Call MarshalIndent with a channel type as input
2. Check the returned error

*Expected:* An error is returned for unsupported data type

#### `TS-json-json-004` — NewEncoder works with valid io.Writer

**P0** · unit · covers `RISK-json-json` · [`NewEncoder` in codec/json/json.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/json.go)

*Steps:*

1. Create a valid io.Writer instance
2. Call NewEncoder with the writer
3. Check that the encoder is not nil

*Expected:* Encoder is successfully created and is not nil

#### `TS-json-json-005` — NewDecoder works with valid io.Reader

**P0** · unit · covers `RISK-json-json` · [`NewDecoder` in codec/json/json.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/json.go)

*Steps:*

1. Create a valid io.Reader instance
2. Call NewDecoder with the reader
3. Check that the decoder is not nil

*Expected:* Decoder is successfully created and is not nil

#### `TS-json-json-006` — Decoder returns an error when reading from an empty io.Reader

**P0** · negative · covers `RISK-json-json` · [`NewDecoder` in codec/json/json.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/json.go)

*Steps:*

1. Create an empty io.Reader
2. Call NewDecoder with the empty reader
3. Attempt to decode from the empty reader
4. Check the returned error

*Expected:* An error is returned indicating that reading failed

### json/jsoniter

#### `TS-json-jsoniter-001` — Marshaling fails when input data is invalid

**P0** · negative · covers `RISK-json-jsoniter` · [`jsoniterApi.Marshal` in codec/json/jsoniter.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/jsoniter.go)

*Steps:*

1. Call Marshal with invalid data (e.g., data that includes circular references)

*Expected:* The function returns an error indicating marshaling failure.

#### `TS-json-jsoniter-002` — Unmarshaling fails when input data is not properly formatted

**P0** · negative · covers `RISK-json-jsoniter` · [`jsoniterApi.Unmarshal` in codec/json/jsoniter.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/jsoniter.go)

*Steps:*

1. Call Unmarshal with improperly formatted JSON data (e.g., missing quotes, trailing commas)

*Expected:* The function returns an error indicating unmarshaling failure.

#### `TS-json-jsoniter-003` — Marshaling with indentation fails when input data is invalid

**P0** · negative · covers `RISK-json-jsoniter` · [`jsoniterApi.MarshalIndent` in codec/json/jsoniter.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/jsoniter.go)

*Steps:*

1. Call MarshalIndent with invalid data (e.g., data that includes circular references)

*Expected:* The function returns an error indicating marshaling with indentation failure.

### json/sonic

#### `TS-json-sonic-001` — Marshal fails with invalid JSON value

**P0** · negative · covers `RISK-json-sonic` · [`sonicApi.Marshal` in codec/json/sonic.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/sonic.go)

*Steps:*

1. Call Marshal with a value that cannot be marshaled (e.g., complex number or function)

*Expected:* An error is returned stating the JSON encoding has failed.

#### `TS-json-sonic-002` — Unmarshal fails with invalid JSON data

**P0** · negative · covers `RISK-json-sonic` · [`sonicApi.Unmarshal` in codec/json/sonic.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/sonic.go)

*Steps:*

1. Call Unmarshal with invalid JSON data (e.g., malformed JSON string)

*Expected:* An error is returned stating the JSON decoding has failed.

#### `TS-json-sonic-003` — MarshalIndent fails with invalid arguments

**P0** · negative · covers `RISK-json-sonic` · [`sonicApi.MarshalIndent` in codec/json/sonic.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/sonic.go)

*Steps:*

1. Call MarshalIndent with invalid prefix or indent arguments (e.g., a nil string)

*Expected:* An error is returned stating the arguments for MarshalIndent are invalid.

#### `TS-json-sonic-004` — NewEncoder fails with a nil writer

**P0** · negative · covers `RISK-json-sonic` · [`sonicApi.NewEncoder` in codec/json/sonic.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/sonic.go)

*Steps:*

1. Call NewEncoder with a nil writer

*Expected:* The function returns an error indicating that the writer is invalid.

#### `TS-json-sonic-005` — NewDecoder fails with a nil reader

**P0** · negative · covers `RISK-json-sonic` · [`sonicApi.NewDecoder` in codec/json/sonic.go](https://github.com/gin-gonic/gin/blob/HEAD/codec/json/sonic.go)

*Steps:*

1. Call NewDecoder with a nil reader

*Expected:* The function returns an error indicating that the reader is invalid.

### logger

#### `TS-logger-001` — Logger handles nil output writer gracefully

**P1** · negative · covers `RISK-logger` · [`LoggerWithWriter` in logger.go](https://github.com/gin-gonic/gin/blob/HEAD/logger.go)

*Preconditions:*

- Logger is instantiated with a nil output writer

*Steps:*

1. Call LoggerWithWriter with a nil output writer
2. Trigger a logging event

*Expected:* No panic occurs, and the system handles the request smoothly.

#### `TS-logger-002` — Logger handles permission denied error on output writer

**P1** · negative · covers `RISK-logger` · [`LoggerWithWriter` in logger.go](https://github.com/gin-gonic/gin/blob/HEAD/logger.go)

*Preconditions:*

- Logger is instantiated with an output writer that has permission issues

*Steps:*

1. Mock the output writer to simulate permission denied error
2. Call LoggerWithWriter passing the mocked writer
3. Trigger a logging event

*Expected:* Logger does not crash, and the logging of the HTTP request fails gracefully with an appropriate error message.

#### `TS-logger-003` — Logger handles JSON marshalling error gracefully

**P1** · negative · covers `RISK-logger` · [`Logger` in logger.go](https://github.com/gin-gonic/gin/blob/HEAD/logger.go)

*Preconditions:*

- Set up a formatter that generates a JSON marshalling error

*Steps:*

1. Instantiate the Logger with the faulty formatter
2. Trigger a logging event

*Expected:* Logger does not crash, returns an error in context errors, and logs the relevant parameters.

#### `TS-logger-004` — Logger with missing context returns nil error message

**P1** · negative · covers `RISK-logger` · [`Logger` in logger.go](https://github.com/gin-gonic/gin/blob/HEAD/logger.go)

*Preconditions:*

- Logger is instantiated with default settings without error context

*Steps:*

1. Call Logger without setting error message in context
2. Trigger a logging event

*Expected:* Logger handles the situation gracefully by returning a nil error message, without crashing.

#### `TS-logger-005` — Logger logs parameters correctly when all inputs are valid

**P1** · unit · covers `RISK-logger` · [`Logger` in logger.go](https://github.com/gin-gonic/gin/blob/HEAD/logger.go)

*Preconditions:*

- Logger is instantiated with a valid output writer and formatter

*Steps:*

1. Call LoggerWithWriter with a valid output writer
2. Trigger a logging event with all valid parameters

*Expected:* Log output contains all parameters including request details, method, status code, and error message formatted correctly.

### mode

#### `TS-mode-001` — SetMode panics on unknown mode

**P1** · negative · covers `RISK-mode` · [`SetMode` in mode.go](https://github.com/gin-gonic/gin/blob/HEAD/mode.go)

*Steps:*

1. Call SetMode with an unknown value, e.g., 'invalid_mode'

*Expected:* Panic is triggered with a message indicating 'gin mode unknown: invalid_mode (available mode: debug release test)'

#### `TS-mode-002` — SetMode sets environment variable GIN_MODE correctly

**P1** · unit · covers `RISK-mode` · [`SetMode` in mode.go](https://github.com/gin-gonic/gin/blob/HEAD/mode.go)

*Preconditions:*

- Set the environment variable GIN_MODE to 'release'

*Steps:*

1. Call SetMode using the environment variable

*Expected:* The current gin mode returns 'release' when calling Mode()

#### `TS-mode-003` — SetMode defaults to DebugMode when GIN_MODE is not set

**P1** · unit · covers `RISK-mode` · [`SetMode` in mode.go](https://github.com/gin-gonic/gin/blob/HEAD/mode.go)

*Steps:*

1. Ensure that GIN_MODE is not set
2. Call SetMode with an empty string

*Expected:* The current gin mode returns 'debug' when calling Mode()

#### `TS-mode-004` — SetMode sets mode to TestMode when in test environment

**P1** · unit · covers `RISK-mode` · [`SetMode` in mode.go](https://github.com/gin-gonic/gin/blob/HEAD/mode.go)

*Preconditions:*

- Simulate the test environment by setting the test.v flag

*Steps:*

1. Call SetMode with an empty string

*Expected:* The current gin mode returns 'test' when calling Mode()

#### `TS-mode-005` — DisableBindValidation closes the default validator

**P1** · unit · covers `RISK-mode` · [`DisableBindValidation` in mode.go](https://github.com/gin-gonic/gin/blob/HEAD/mode.go)

*Steps:*

1. Call DisableBindValidation()
2. Check if binding.Validator is nil

*Expected:* binding.Validator is nil

#### `TS-mode-006` — EnableJsonDecoderUseNumber sets binding.EnableDecoderUseNumber to true

**P1** · unit · covers `RISK-mode` · [`EnableJsonDecoderUseNumber` in mode.go](https://github.com/gin-gonic/gin/blob/HEAD/mode.go)

*Steps:*

1. Call EnableJsonDecoderUseNumber()
2. Check if binding.EnableDecoderUseNumber is true

*Expected:* binding.EnableDecoderUseNumber is true

#### `TS-mode-007` — EnableJsonDecoderDisallowUnknownFields sets binding.EnableDecoderDisallowUnknownFields to true

**P1** · unit · covers `RISK-mode` · [`EnableJsonDecoderDisallowUnknownFields` in mode.go](https://github.com/gin-gonic/gin/blob/HEAD/mode.go)

*Steps:*

1. Call EnableJsonDecoderDisallowUnknownFields()
2. Check if binding.EnableDecoderDisallowUnknownFields is true

*Expected:* binding.EnableDecoderDisallowUnknownFields is true

### recovery

#### `TS-recovery-001` — Recovery middleware handles panic and logs error

**P1** · e2e · covers `RISK-recovery` · [`recovery.go`](https://github.com/gin-gonic/gin/blob/HEAD/recovery.go)

*Steps:*

1. Create a Gin router using gin.Default()
2. Add a recovery middleware to the router
3. Set up a route that triggers a panic
4. Start the server in a goroutine
5. Make a request to the route that panics
6. Check the log output for a message indicating panic recovery

*Expected:* Log contains a panic recovery message with the relevant details.

#### `TS-recovery-002` — Recovery middleware logs error when unable to write to log output

**P1** · negative · covers `RISK-recovery` · [`recovery.go`](https://github.com/gin-gonic/gin/blob/HEAD/recovery.go)

*Steps:*

1. Create a Gin router using gin.Default()
2. Setup a route that triggers a panic
3. Configure the recovery middleware to use an invalid log writer
4. Make a request to the route that panics
5. Check the application response for status 500

*Expected:* Application response is a 500 Internal Server Error.

#### `TS-recovery-003` — Recovery middleware handles an error in reading the nth line from a file

**P1** · negative · covers `RISK-recovery` · [`recovery.go`](https://github.com/gin-gonic/gin/blob/HEAD/recovery.go)

*Steps:*

1. Create a Gin router using gin.Default()
2. Set up a route that triggers a panic that eventually requires reading a line from a non-existing file
3. Make a request to the route that panics
4. Check the application response for status 500

*Expected:* Application response is a 500 Internal Server Error.

#### `TS-recovery-004` — Recovery middleware handles error opening a file

**P1** · negative · covers `RISK-recovery` · [`recovery.go`](https://github.com/gin-gonic/gin/blob/HEAD/recovery.go)

*Steps:*

1. Create a Gin router using gin.Default()
2. Set up a route that triggers a panic that requires file access
3. Simulate an error opening a file (mocking required)
4. Make a request to the route that panics
5. Check the application response for status 500

*Expected:* Application response is a 500 Internal Server Error.

#### `TS-recovery-005` — Recovery middleware fails gracefully when opening a non-existing file

**P1** · negative · covers `RISK-recovery` · [`recovery.go`](https://github.com/gin-gonic/gin/blob/HEAD/recovery.go)

*Steps:*

1. Call readNthLine with a path to a non-existing file.

*Expected:* The function returns an error indicating that the file could not be opened.

### render/bson

#### `TS-render-bson-001` — Render returns error if BSON marshaling fails

**P1** · unit · covers `RISK-render-bson` · [`BSON.Render` in render/bson.go](https://github.com/gin-gonic/gin/blob/HEAD/render/bson.go)

*Steps:*

1. Create a BSON object with invalid data for marshaling (e.g., a channel or function)
2. Create a mock HTTP response writer
3. Call the Render method of the BSON object with the mock response writer

*Expected:* The method returns an error indicating BSON marshaling failure.

#### `TS-render-bson-002` — Render handles successful BSON marshaling and HTTP response writing

**P1** · unit · covers `RISK-render-bson` · [`BSON.Render` in render/bson.go](https://github.com/gin-gonic/gin/blob/HEAD/render/bson.go)

*Steps:*

1. Create a BSON object with valid data for marshaling
2. Create a mock HTTP response writer
3. Call the Render method of the BSON object with the mock response writer

*Expected:* The method successfully writes BSON data to the HTTP response without returning an error.

#### `TS-render-bson-003` — Render returns error if HTTP response writing fails

**P1** · unit · covers `RISK-render-bson` · [`BSON.Render` in render/bson.go](https://github.com/gin-gonic/gin/blob/HEAD/render/bson.go)

*Steps:*

1. Create a BSON object with valid data for marshaling
2. Create a mock HTTP response writer that simulates an error on Write (e.g., by wrapping the writer with a custom writer that returns an error)
3. Call the Render method of the BSON object with the mock response writer

*Expected:* The method returns an error indicating HTTP response writing failure.

### render/html

#### `TS-render-html-001` — Render fails when template is missing

**P0** · negative · covers `RISK-render-html` · [`HTML.Render` in render/html.go](https://github.com/gin-gonic/gin/blob/HEAD/render/html.go)

*Steps:*

1. Create an HTML instance with a missing template name
2. Call the Render function with a valid http.ResponseWriter

*Expected:* The function returns an error indicating template not found.

#### `TS-render-html-002` — Render fails when template execution fails

**P0** · negative · covers `RISK-render-html` · [`HTML.Render` in render/html.go](https://github.com/gin-gonic/gin/blob/HEAD/render/html.go)

*Steps:*

1. Create an HTML instance with a valid template name that fails to execute
2. Call the Render function with a valid http.ResponseWriter

*Expected:* The function returns an error indicating template execution failure.

#### `TS-render-html-003` — Render fails when http.ResponseWriter is nil

**P0** · negative · covers `RISK-render-html` · [`HTML.Render` in render/html.go](https://github.com/gin-gonic/gin/blob/HEAD/render/html.go)

*Steps:*

1. Create an HTML instance with a valid template name
2. Call the Render function with a nil http.ResponseWriter

*Expected:* The function returns an error indicating invalid ResponseWriter.

#### `TS-render-html-004` — Render sets the correct Content-Type header

**P0** · unit · covers `RISK-render-html` · [`HTML.WriteContentType` in render/html.go](https://github.com/gin-gonic/gin/blob/HEAD/render/html.go)

*Steps:*

1. Create an HTML instance
2. Call the WriteContentType function with a mock http.ResponseWriter
3. Check the headers of the mock ResponseWriter

*Expected:* The Content-Type header is set to 'text/html; charset=utf-8'.

#### `TS-render-html-005` — LoadTemplate fails when no files or patterns are provided

**P0** · negative · covers `RISK-render-html` · [`HTMLDebug.loadTemplate` in render/html.go](https://github.com/gin-gonic/gin/blob/HEAD/render/html.go)

*Steps:*

1. Create an instance of HTMLDebug without Files, Glob, or Patterns
2. Call the loadTemplate method

*Expected:* loadTemplate panics with 'the HTML debug render was created without files or glob pattern or file system with patterns'.

### render/json

#### `TS-render-json-001` — Test JSON rendering failure when marshalling data to JSON

**P0** · negative · covers `RISK-render-json` · [`WriteJSON` in render/json.go](https://github.com/gin-gonic/gin/blob/HEAD/render/json.go)

*Steps:*

1. Instantiate a JSON object with invalid data that cannot be marshalled (e.g., a channel)
2. Create a mock http.ResponseWriter
3. Call the WriteJSON function with the mock ResponseWriter and the invalid JSON object

*Expected:* The function returns an error indicating the failure to marshal the data.

#### `TS-render-json-002` — Test JSON rendering failure when writing to http.ResponseWriter

**P0** · negative · covers `RISK-render-json` · [`WriteJSON` in render/json.go](https://github.com/gin-gonic/gin/blob/HEAD/render/json.go)

*Steps:*

1. Create a JSON object with valid data
2. Create a mock http.ResponseWriter that simulates a write failure (e.g., return an error on Write method)
3. Call the WriteJSON function with the mock ResponseWriter and the valid JSON object

*Expected:* The function returns an error indicating the write failure to the http.ResponseWriter.

#### `TS-render-json-003` — Test SecureJSON rendering failure when marshaling data to JSON

**P0** · negative · covers `RISK-render-json` · [`SecureJSON.Render` in render/json.go](https://github.com/gin-gonic/gin/blob/HEAD/render/json.go)

*Steps:*

1. Instantiate a SecureJSON object with invalid data that cannot be marshalled (e.g., a channel)
2. Create a mock http.ResponseWriter
3. Call the Render method with the mock ResponseWriter

*Expected:* The method returns an error indicating the failure to marshal the data.

#### `TS-render-json-004` — Test JsonpJSON rendering failure when a callback is escaped and the Write fails

**P0** · negative · covers `RISK-render-json` · [`JsonpJSON.Render` in render/json.go](https://github.com/gin-gonic/gin/blob/HEAD/render/json.go)

*Steps:*

1. Instantiate a JsonpJSON object with valid data and a valid callback
2. Create a mock http.ResponseWriter that simulates a write failure
3. Call the Render method with the mock ResponseWriter

*Expected:* The method returns an error indicating the write failure.

#### `TS-render-json-005` — Test that WriteJSON writes correct content type to http.ResponseWriter

**P0** · unit · covers `RISK-render-json` · [`WriteJSON` in render/json.go](https://github.com/gin-gonic/gin/blob/HEAD/render/json.go)

*Steps:*

1. Create a valid JSON object
2. Create a mock http.ResponseWriter that captures written data
3. Call the WriteJSON function with the mock ResponseWriter and the valid JSON object
4. Inspect the written content type in the mock ResponseWriter

*Expected:* The correct JSON content type 'application/json; charset=utf-8' is written.

#### `TS-render-json-006` — Test that JsonpJSON writes correct content type to http.ResponseWriter

**P0** · unit · covers `RISK-render-json` · [`JsonpJSON.WriteContentType` in render/json.go](https://github.com/gin-gonic/gin/blob/HEAD/render/json.go)

*Steps:*

1. Create a JsonpJSON object with a valid callback
2. Create a mock http.ResponseWriter that captures written data
3. Call the WriteContentType method with the mock ResponseWriter
4. Inspect the written content type in the mock ResponseWriter

*Expected:* The correct JSONP content type 'application/javascript; charset=utf-8' is written.

#### `TS-render-json-007` — Render correctly writes JSONP with callback

**P0** · unit · covers `RISK-render-json` · [`Render` in render/json.go](https://github.com/gin-gonic/gin/blob/HEAD/render/json.go)

*Preconditions:*

- Setup a mock http.ResponseWriter

*Steps:*

1. Create an instance of JsonpJSON with a valid callback and data.
2. Call the Render method, passing the mocked ResponseWriter.

*Expected:* The ResponseWriter should receive the appropriate JSONP format.

#### `TS-render-json-008` — Render returns an error if HTTP write fails for JSONP

**P0** · negative · covers `RISK-render-json` · [`Render` in render/json.go](https://github.com/gin-gonic/gin/blob/HEAD/render/json.go)

*Preconditions:*

- Setup a mock http.ResponseWriter that simulates a write failure.

*Steps:*

1. Create an instance of JsonpJSON with valid callback and data.
2. Call the Render method, passing the mocked ResponseWriter.

*Expected:* The Render method should return an error indicating the HTTP write failure.

#### `TS-render-json-009` — Render correctly escapes callback in JSONP

**P0** · unit · covers `RISK-render-json` · [`Render` in render/json.go](https://github.com/gin-gonic/gin/blob/HEAD/render/json.go)

*Preconditions:*

- Setup a mock http.ResponseWriter

*Steps:*

1. Create an instance of JsonpJSON with a callback containing special characters.
2. Call the Render method, passing the mocked ResponseWriter.

*Expected:* The ResponseWriter should receive JSONP with the callback correctly escaped.

#### `TS-render-json-010` — Render returns an error if JSON marshalling fails for JsonpJSON

**P0** · negative · covers `RISK-render-json` · [`Render` in render/json.go](https://github.com/gin-gonic/gin/blob/HEAD/render/json.go)

*Preconditions:*

- Setup a mock http.ResponseWriter.

*Steps:*

1. Create an instance of JsonpJSON with data designed to fail marshaling.
2. Call the Render method, passing the mocked ResponseWriter.

*Expected:* The Render method should return an error indicating marshaling failure.

### render/msgpack

#### `TS-render-msgpack-001` — Render returns an error when encoding fails

**P1** · negative · covers `RISK-render-msgpack` · [`Render` in render/msgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/render/msgpack.go)

*Preconditions:*

- A MsgPack object with data that cannot be encoded

*Steps:*

1. Create a MsgPack instance with unencodable data (e.g., a channel or a function)
2. Create an http.ResponseWriter mock that captures the output
3. Call the Render method of MsgPack with the http.ResponseWriter mock

*Expected:* The Render method returns an error indicating encoding failure.

#### `TS-render-msgpack-002` — WriteContentType fails when response writer fails

**P1** · negative · covers `RISK-render-msgpack` · [`WriteContentType` in render/msgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/render/msgpack.go)

*Preconditions:*

- Mock an HTTP response writer that returns an error on write

*Steps:*

1. Create a MsgPack instance
2. Create a mock http.ResponseWriter that simulates a write failure
3. Call the WriteContentType method of MsgPack with the mock ResponseWriter

*Expected:* The WriteContentType method does not panic and handles the write error gracefully.

#### `TS-render-msgpack-003` — WriteMsgPack returns an error when encoding fails

**P1** · negative · covers `RISK-render-msgpack` · [`WriteMsgPack` in render/msgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/render/msgpack.go)

*Preconditions:*

- A MsgPack object with data that cannot be encoded

*Steps:*

1. Create a MsgPack instance with unencodable data (e.g., a channel or a function)
2. Create an http.ResponseWriter mock that captures the output
3. Call the WriteMsgPack function with the http.ResponseWriter mock and the MsgPack data

*Expected:* WriteMsgPack returns an error indicating encoding failure.

#### `TS-render-msgpack-004` — Server responds with correct ContentType for MsgPack

**P1** · unit · covers `RISK-render-msgpack` · [`Render` in render/msgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/render/msgpack.go)

*Preconditions:*

- A valid MsgPack object with encodable data

*Steps:*

1. Create a MsgPack instance with valid data
2. Create an http.ResponseWriter mock that captures the output
3. Call the Render method of MsgPack with the mock ResponseWriter

*Expected:* The ContentType written to the mock ResponseWriter is 'application/msgpack; charset=utf-8'.

#### `TS-render-msgpack-005` — Server does not write ContentType if WriteContentType fails

**P1** · negative · covers `RISK-render-msgpack` · [`WriteContentType` in render/msgpack.go](https://github.com/gin-gonic/gin/blob/HEAD/render/msgpack.go)

*Preconditions:*

- Mock an HTTP response writer that returns an error on write

*Steps:*

1. Create a MsgPack instance
2. Create a mock http.ResponseWriter that simulates a write failure
3. Call the WriteContentType method of MsgPack with the mock ResponseWriter

*Expected:* The ResponseWriter does not save the ContentType due to the write error.

### render/protobuf

#### `TS-render-protobuf-001` — Render fails when marshalling the data to Protocol Buffers format

**P1** · negative · covers `RISK-render-protobuf` · [`ProtoBuf.Render` in render/protobuf.go](https://github.com/gin-gonic/gin/blob/HEAD/render/protobuf.go)

*Steps:*

1. Create an instance of ProtoBuf with invalid data that does not implement proto.Message
2. Call the Render method with a mock HTTP response writer

*Expected:* The Render method returns an error indicating the failure to marshal the data.

#### `TS-render-protobuf-002` — Render fails when writing to the HTTP response

**P1** · negative · covers `RISK-render-protobuf` · [`ProtoBuf.Render` in render/protobuf.go](https://github.com/gin-gonic/gin/blob/HEAD/render/protobuf.go)

*Preconditions:*

- Set up a mock HTTP response writer that fails on write

*Steps:*

1. Create an instance of ProtoBuf with valid data that implements proto.Message
2. Call the Render method with the mock HTTP response writer

*Expected:* The Render method returns an error indicating the failure to write to the HTTP response.

#### `TS-render-protobuf-003` — Render successfully writes valid data to HTTP response

**P1** · unit · covers `RISK-render-protobuf` · [`ProtoBuf.Render` in render/protobuf.go](https://github.com/gin-gonic/gin/blob/HEAD/render/protobuf.go)

*Steps:*

1. Create an instance of ProtoBuf with valid data that implements proto.Message
2. Call the Render method with a mock HTTP response writer that captures output

*Expected:* The HTTP response writer receives the correct bytes of the marshalled Protocol Buffers data.

#### `TS-render-protobuf-004` — WriteContentType correctly writes the content type header

**P1** · unit · covers `RISK-render-protobuf` · [`ProtoBuf.WriteContentType` in render/protobuf.go](https://github.com/gin-gonic/gin/blob/HEAD/render/protobuf.go)

*Steps:*

1. Create an instance of ProtoBuf
2. Call the WriteContentType method with a mock HTTP response writer

*Expected:* The HTTP response writer has the correct content type header set to 'application/x-protobuf'.

### render/reader

#### `TS-render-reader-001` — Render fails when Reader is nil

**P1** · negative · covers `RISK-render-reader` · [`Reader.Render` in render/reader.go](https://github.com/gin-gonic/gin/blob/HEAD/render/reader.go)

*Steps:*

1. Create a Reader instance with a nil Reader field
2. Create a mock http.ResponseWriter
3. Call Render on the Reader instance with the mock ResponseWriter

*Expected:* An error is returned indicating that the Reader cannot be nil.

#### `TS-render-reader-002` — Render fails on network error during io.Copy

**P1** · negative · covers `RISK-render-reader` · [`Reader.Render` in render/reader.go](https://github.com/gin-gonic/gin/blob/HEAD/render/reader.go)

*Steps:*

1. Create a Reader instance with valid ContentType and Headers
2. Create a mock http.ResponseWriter that simulates a network error when writing
3. Call Render on the Reader instance with the mock ResponseWriter

*Expected:* An error is returned indicating a network write error.

#### `TS-render-reader-003` — Render fails to write headers due to invalid header value

**P1** · negative · covers `RISK-render-reader` · [`Reader.Render` in render/reader.go](https://github.com/gin-gonic/gin/blob/HEAD/render/reader.go)

*Steps:*

1. Create a Reader instance with a header containing an invalid value
2. Create a mock http.ResponseWriter
3. Call Render on the Reader instance with the mock ResponseWriter

*Expected:* An error is returned indicating that the header value is invalid.

#### `TS-render-reader-004` — Render writes correct Content-Type header

**P1** · unit · covers `RISK-render-reader` · [`Reader.Render` in render/reader.go](https://github.com/gin-gonic/gin/blob/HEAD/render/reader.go)

*Steps:*

1. Create a Reader instance with a valid ContentType
2. Create a mock http.ResponseWriter
3. Call Render on the Reader instance with the mock ResponseWriter
4. Check if the Content-Type header is set correctly in the mock ResponseWriter

*Expected:* The Content-Type header in the ResponseWriter matches the ContentType of the Reader.

#### `TS-render-reader-005` — Render writes calculated Content-Length header

**P1** · unit · covers `RISK-render-reader` · [`Reader.Render` in render/reader.go](https://github.com/gin-gonic/gin/blob/HEAD/render/reader.go)

*Steps:*

1. Create a Reader instance with a valid ContentLength
2. Create a mock http.ResponseWriter
3. Call Render on the Reader instance with the mock ResponseWriter
4. Check the Content-Length header in the mock ResponseWriter

*Expected:* The Content-Length header in the ResponseWriter matches the ContentLength of the Reader.

### render/text

#### `TS-render-text-001` — Render returns error when unable to write response

**P1** · negative · covers `RISK-render-text` · [`Render` in render/text.go](https://github.com/gin-gonic/gin/blob/HEAD/render/text.go)

*Preconditions:*

- Set up a mock ResponseWriter that returns an error on Write

*Steps:*

1. Create a String instance with a valid format and data
2. Invoke the Render method on the String instance with the mock ResponseWriter

*Expected:* Render method returns an error indicating failure to write response.

#### `TS-render-text-002` — Render returns error for invalid format string

**P1** · negative · covers `RISK-render-text` · [`Render` in render/text.go](https://github.com/gin-gonic/gin/blob/HEAD/render/text.go)

*Steps:*

1. Create a String instance with an invalid format string
2. Invoke the Render method on the String instance with a valid ResponseWriter

*Expected:* Render method returns an error indicating an invalid format.

#### `TS-render-text-003` — Render writes plain content type if no data is provided

**P1** · unit · covers `RISK-render-text` · [`Render` in render/text.go](https://github.com/gin-gonic/gin/blob/HEAD/render/text.go)

*Steps:*

1. Create a String instance with a valid format and empty data slice
2. Set up a mock ResponseWriter
3. Invoke the Render method on the String instance with the mock ResponseWriter
4. Check if Content-Type header is set to 'text/plain; charset=utf-8'

*Expected:* ResponseWriter Content-Type header is 'text/plain; charset=utf-8'.

#### `TS-render-text-004` — WriteString handles empty data gracefully

**P1** · unit · covers `RISK-render-text` · [`WriteString` in render/text.go](https://github.com/gin-gonic/gin/blob/HEAD/render/text.go)

*Steps:*

1. Create a mock ResponseWriter
2. Call WriteString with valid format and empty data slice

*Expected:* No errors returned, and Write is invoked on ResponseWriter with format string.

#### `TS-render-text-005` — Render does not write data when no data is provided

**P1** · unit · covers `RISK-render-text` · [`Render` in render/text.go](https://github.com/gin-gonic/gin/blob/HEAD/render/text.go)

*Steps:*

1. Set up a mock ResponseWriter
2. Create a String instance with a valid format and empty data
3. Invoke the Render method on the String instance with the mock ResponseWriter
4. Verify that Write is called with the format string instead of formatted data.

*Expected:* Write method of ResponseWriter is called with an unformatted version of the format string.

### render/xml

#### `TS-render-xml-001` — Render returns error for XML encoding failure

**P1** · negative · covers `RISK-render-xml` · [`Render` in render/xml.go](https://github.com/gin-gonic/gin/blob/HEAD/render/xml.go)

*Preconditions:*

- A response writer that simulates an error during write

*Steps:*

1. Create a mock response writer that returns an error on Write
2. Create an instance of XML with data to encode
3. Call the Render method with the mock response writer

*Expected:* Receive an error indicating XML encoding failure.

#### `TS-render-xml-002` — Render executes without error for valid XML data

**P1** · unit · covers `RISK-render-xml` · [`Render` in render/xml.go](https://github.com/gin-gonic/gin/blob/HEAD/render/xml.go)

*Steps:*

1. Create a mock response writer
2. Create an instance of XML with valid data to encode
3. Call the Render method with the mock response writer

*Expected:* The writer receives valid XML data without any errors.

#### `TS-render-xml-003` — Render sets correct Content-Type header for XML

**P1** · unit · covers `RISK-render-xml` · [`Render` in render/xml.go](https://github.com/gin-gonic/gin/blob/HEAD/render/xml.go)

*Steps:*

1. Create a mock response writer
2. Create an instance of XML with valid data to encode
3. Call the Render method with the mock response writer
4. Check the headers of the response writer

*Expected:* Content-Type header is set to 'application/xml; charset=utf-8'.

#### `TS-render-xml-004` — Render handles HTTP write failure gracefully

**P1** · negative · covers `RISK-render-xml` · [`Render` in render/xml.go](https://github.com/gin-gonic/gin/blob/HEAD/render/xml.go)

*Preconditions:*

- A response writer that simulates an HTTP write error

*Steps:*

1. Create a mock response writer that returns an error on Write
2. Create an instance of XML with valid data to encode
3. Call the Render method with the mock response writer

*Expected:* Receive an error indicating HTTP write failure.

### response_writer

#### `TS-response-writer-001` — ResponseWriter.WriteHeader does not change status after body written

**P0** · negative · covers `RISK-response-writer` · [`WriteHeader` in response_writer.go](https://github.com/gin-gonic/gin/blob/HEAD/response_writer.go)

*Preconditions:*

- A responseWriter instance with status code set
- The response body has already been written

*Steps:*

1. Call WriteHeader with a new status code

*Expected:* The status code remains unchanged and no warning is logged.

#### `TS-response-writer-002` — ResponseWriter.WriteHeader writes header with valid status

**P0** · unit · covers `RISK-response-writer` · [`WriteHeader` in response_writer.go](https://github.com/gin-gonic/gin/blob/HEAD/response_writer.go)

*Preconditions:*

- A responseWriter instance with no body written

*Steps:*

1. Call WriteHeader with a valid status code

*Expected:* The status code is updated and can be retrieved using the Status method.

#### `TS-response-writer-003` — ResponseWriter.Hijack fails after body written

**P0** · negative · covers `RISK-response-writer` · [`Hijack` in response_writer.go](https://github.com/gin-gonic/gin/blob/HEAD/response_writer.go)

*Preconditions:*

- A responseWriter instance with body data written

*Steps:*

1. Call Hijack method

*Expected:* An error is returned indicating that hijacking is not allowed because data has been written.

#### `TS-response-writer-004` — ResponseWriter.Hijack succeeds before body written

**P0** · unit · covers `RISK-response-writer` · [`Hijack` in response_writer.go](https://github.com/gin-gonic/gin/blob/HEAD/response_writer.go)

*Preconditions:*

- A responseWriter instance with no body written

*Steps:*

1. Call Hijack method

*Expected:* The method returns a net.Conn and *bufio.ReadWriter without errors.

#### `TS-response-writer-005` — ResponseWriter.Flush on unsupported writer

**P0** · negative · covers `RISK-response-writer` · [`Flush` in response_writer.go](https://github.com/gin-gonic/gin/blob/HEAD/response_writer.go)

*Preconditions:*

- A responseWriter instance of a writer that doesn't implement http.Flusher

*Steps:*

1. Call Flush method

*Expected:* The method does not panic and no flush occurs.

#### `TS-response-writer-006` — ResponseWriter.CloseNotify on unsupported writer

**P0** · negative · covers `RISK-response-writer` · [`CloseNotify` in response_writer.go](https://github.com/gin-gonic/gin/blob/HEAD/response_writer.go)

*Preconditions:*

- A responseWriter instance of a writer that doesn't implement http.CloseNotifier

*Steps:*

1. Call CloseNotify method

*Expected:* The method returns nil without any panic.

### routergroup

#### `TS-routergroup-001` — Handle rejects invalid HTTP methods

**P0** · negative · covers `RISK-routergroup` · [`RouterGroup.Handle` in routergroup.go](https://github.com/gin-gonic/gin/blob/HEAD/routergroup.go)

*Steps:*

1. Create a new RouterGroup instance
2. Call Handle with an invalid HTTP method (e.g., 'INVALID_METHOD')
3. Expect a panic to occur

*Expected:* The process should panic with the message 'http method INVALID_METHOD is not valid'

#### `TS-routergroup-002` — Group method correctly calculates base path

**P0** · unit · covers `RISK-routergroup` · [`RouterGroup.Group` in routergroup.go](https://github.com/gin-gonic/gin/blob/HEAD/routergroup.go)

*Steps:*

1. Create a new RouterGroup instance with a base path
2. Call Group with a relative path
3. Get the base path from the RouterGroup using BasePath
4. Verify that the returned base path is the correct concatenation of the base path and the relative path

*Expected:* The base path should match the expected concatenation of the initial base path and the relative path

#### `TS-routergroup-003` — StaticFile handler fails when URL params are used

**P0** · negative · covers `RISK-routergroup` · [`RouterGroup.StaticFile` in routergroup.go](https://github.com/gin-gonic/gin/blob/HEAD/routergroup.go)

*Steps:*

1. Create a new RouterGroup instance
2. Attempt to register a static file handler using a relativePath that contains a parameter (e.g., /static/:id)
3. Expect a panic to occur

*Expected:* The process should panic with the message 'URL parameters can not be used when serving a static file'

#### `TS-routergroup-004` — StaticFS handler rejects URL parameters in path

**P0** · negative · covers `RISK-routergroup` · [`RouterGroup.StaticFS` in routergroup.go](https://github.com/gin-gonic/gin/blob/HEAD/routergroup.go)

*Steps:*

1. Create a new RouterGroup instance
2. Attempt to register StaticFS with a relativePath that contains a parameter (e.g., '/static/*filepath')
3. Expect a panic to occur

*Expected:* The process should panic with the message 'URL parameters can not be used when serving a static folder'

#### `TS-routergroup-005` — Check for file not found response in StaticFile handler

**P0** · negative · covers `RISK-routergroup` · [`RouterGroup.StaticFile` in routergroup.go](https://github.com/gin-gonic/gin/blob/HEAD/routergroup.go)

*Steps:*

1. Create a new RouterGroup instance with StaticFile that points to a non-existent file
2. Simulate an HTTP request to the registered route
3. Verify the HTTP response status code

*Expected:* The response status code should be 404 Not Found

#### `TS-routergroup-006` — Validate proper registration of routes with HTTP methods

**P0** · unit · covers `RISK-routergroup` · [`RouterGroup.Handle` in routergroup.go](https://github.com/gin-gonic/gin/blob/HEAD/routergroup.go)

*Steps:*

1. Create a new RouterGroup instance
2. Register a route with a valid HTTP method and handler
3. Simulate an HTTP request with that HTTP method to the registered route
4. Verify the expected handler is executed

*Expected:* The expected handler should respond correctly for the registered route with the specified HTTP method

### test_helpers

#### `TS-test-helpers-001` — Ensure error is returned when engine creation fails

**P1** · negative · covers `RISK-test-helpers` · [`EngineCreation` in test_helpers.go](https://github.com/gin-gonic/gin/blob/HEAD/test_helpers.go)

*Steps:*

1. Call the function responsible for engine creation with parameters that are guaranteed to lead to an error.

*Expected:* The function returns an error indicating engine creation has failed.

#### `TS-test-helpers-002` — Verify context allocation failure is handled gracefully

**P1** · negative · covers `RISK-test-helpers` · [`ContextAllocation` in test_helpers.go](https://github.com/gin-gonic/gin/blob/HEAD/test_helpers.go)

*Steps:*

1. Invoke the function that allocates a context with invalid parameters.
2. Check if the function returns an error without crashing.

*Expected:* The function should return an error related to context allocation failure.

#### `TS-test-helpers-003` — Check HTTP request failure handling

**P1** · negative · covers `RISK-test-helpers` · [`HTTPRequest` in test_helpers.go](https://github.com/gin-gonic/gin/blob/HEAD/test_helpers.go)

*Steps:*

1. Make an HTTP request to a server that is guaranteed to be unreachable.
2. Assert that the function returns an appropriate error message.

*Expected:* The function should return an error indicating that the HTTP request failed.

#### `TS-test-helpers-004` — Test the exponential backoff failure handling

**P1** · negative · covers `RISK-test-helpers` · [`ExponentialBackoff` in test_helpers.go](https://github.com/gin-gonic/gin/blob/HEAD/test_helpers.go)

*Steps:*

1. Attempt to trigger a failure in the exponential backoff mechanism by simulating repeated failures during server readiness checks.
2. Verify that the function does not enter an infinite loop and returns an error.

*Expected:* The function should return an error after the designated number of retries, indicating that backoff has failed.

### tree

#### `TS-tree-001` — Add route with a wildcard in the middle of the path that conflicts with an existing wildcard

**P1** · negative · covers `RISK-tree` · [`addRoute` in tree.go](https://github.com/gin-gonic/gin/blob/HEAD/tree.go)

*Steps:*

1. Create a new node.
2. Add a route with a path that contains one wildcard (e.g., /user/:name).
3. Add a second route with a conflicting path (e.g., /user/:name/details).

*Expected:* Panic with the message indicating a wildcard conflict.

#### `TS-tree-002` — Add a catch-all wildcard in the middle of the path

**P1** · negative · covers `RISK-tree` · [`addRoute` in tree.go](https://github.com/gin-gonic/gin/blob/HEAD/tree.go)

*Steps:*

1. Create a new node.
2. Add a route with a path (e.g., /user/:name).
3. Add a route with a catch-all in the middle (e.g., /user/*path/details).

*Expected:* Panic with the message indicating catch-all conflicts with existing path.

#### `TS-tree-003` — Add a second wildcard after the first one

**P1** · negative · covers `RISK-tree` · [`addRoute` in tree.go](https://github.com/gin-gonic/gin/blob/HEAD/tree.go)

*Steps:*

1. Create a new node.
2. Add a route with a first wildcard path (e.g., /user/:name).
3. Add a route with a second wildcard path (e.g., /user/:name/profile).

*Expected:* Panic with a message stating that only one wildcard is allowed per path segment.

#### `TS-tree-004` — Add a route with an empty wildcard name

**P1** · negative · covers `RISK-tree` · [`addRoute` in tree.go](https://github.com/gin-gonic/gin/blob/HEAD/tree.go)

*Steps:*

1. Create a new node.
2. Add a route with a path containing an empty wildcard (e.g., /user/:/details).

*Expected:* Panic with a message stating wildcards must have a non-empty name.

#### `TS-tree-005` — Add a route with an invalid escape sequence

**P1** · negative · covers `RISK-tree` · [`addRoute` in tree.go](https://github.com/gin-gonic/gin/blob/HEAD/tree.go)

*Steps:*

1. Create a new node.
2. Add a route with an invalid escape in the path (e.g., /user/\:name).

*Expected:* Panic with a message indicating an invalid escape sequence.

#### `TS-tree-006` — Register duplicate handlers for the same path

**P1** · negative · covers `RISK-tree` · [`addRoute` in tree.go](https://github.com/gin-gonic/gin/blob/HEAD/tree.go)

*Steps:*

1. Create a new node.
2. Add a route with a specific path (e.g., /user).
3. Attempt to add the same route with a different handler.

*Expected:* Panic indicating that handlers are already registered for the path.

### utils

#### `TS-utils-001` — Bind function returns an error when binding a nil pointer

**P0** · negative · covers `RISK-utils` · [`Bind` in utils.go](https://github.com/gin-gonic/gin/blob/HEAD/utils.go)

*Steps:*

1. Create a request with body data that represents a struct
2. Call the Bind function with a nil pointer as the destination

*Expected:* The Bind function returns an error indicating the destination is nil.

#### `TS-utils-002` — MarshalXML function returns an error during XML marshaling due to invalid input

**P0** · negative · covers `RISK-utils` · [`MarshalXML` in utils.go](https://github.com/gin-gonic/gin/blob/HEAD/utils.go)

*Steps:*

1. Create a struct that causes XML marshaling to fail
2. Call the MarshalXML function with this struct.

*Expected:* The MarshalXML function returns an error indicating failure to marshal the struct.

#### `TS-utils-003` — lastChar function panics when provided with an empty string

**P0** · negative · covers `RISK-utils` · [`lastChar` in utils.go](https://github.com/gin-gonic/gin/blob/HEAD/utils.go)

*Steps:*

1. Call lastChar with an empty string

*Expected:* The program panics as expected with an appropriate error message.

#### `TS-utils-004` — resolveAddress function panics when provided with too many parameters

**P0** · negative · covers `RISK-utils` · [`resolveAddress` in utils.go](https://github.com/gin-gonic/gin/blob/HEAD/utils.go)

*Steps:*

1. Call the resolveAddress function with more than the expected number of parameters

*Expected:* The program panics as expected with an appropriate error message.

#### `TS-utils-005` — chooseData function panics when both parameters are nil

**P0** · negative · covers `RISK-utils` · [`chooseData` in utils.go](https://github.com/gin-gonic/gin/blob/HEAD/utils.go)

*Steps:*

1. Call chooseData with both parameters set to nil

*Expected:* The program panics as expected with an appropriate error message.

## Traceability

| Scenario | Symbol | Risk | Priority |
|---|---|---|---|
| `TS-auth-001` | `BasicAuthForRealm` | `RISK-auth` | P0 |
| `TS-auth-002` | `BasicAuthForRealm` | `RISK-auth` | P0 |
| `TS-auth-003` | `BasicAuthForRealm` | `RISK-auth` | P0 |
| `TS-auth-004` | `BasicAuthForRealm` | `RISK-auth` | P0 |
| `TS-auth-005` | `BasicAuthForProxy` | `RISK-auth` | P0 |
| `TS-auth-006` | `BasicAuthForProxy` | `RISK-auth` | P0 |
| `TS-auth-007` | `BasicAuthForProxy` | `RISK-auth` | P0 |
| `TS-auth-008` | `BasicAuthForProxy` | `RISK-auth` | P0 |
| `TS-binding-binding-001` | `Default` | `RISK-binding-binding` | P1 |
| `TS-binding-binding-002` | `Default` | `RISK-binding-binding` | P1 |
| `TS-binding-binding-003` | `Default` | `RISK-binding-binding` | P1 |
| `TS-binding-binding-004` | `validate` | `RISK-binding-binding` | P1 |
| `TS-binding-binding-005` | `Default` | `RISK-binding-binding` | P1 |
| `TS-binding-binding-006` | `validate` | `RISK-binding-binding` | P1 |
| `TS-binding-binding-nomsgpack-001` | `validate` | `RISK-binding-binding-nomsgpack` | P1 |
| `TS-binding-binding-nomsgpack-002` | `Default` | `RISK-binding-binding-nomsgpack` | P1 |
| `TS-binding-binding-nomsgpack-003` | `Default` | `RISK-binding-binding-nomsgpack` | P1 |
| `TS-binding-binding-nomsgpack-004` | `Bind` | `RISK-binding-binding-nomsgpack` | P1 |
| `TS-binding-binding-nomsgpack-005` | `validate` | `RISK-binding-binding-nomsgpack` | P1 |
| `TS-binding-binding-nomsgpack-006` | `Default` | `RISK-binding-binding-nomsgpack` | P1 |
| `TS-binding-binding-nomsgpack-007` | `Default` | `RISK-binding-binding-nomsgpack` | P1 |
| `TS-binding-binding-nomsgpack-008` | `Default` | `RISK-binding-binding-nomsgpack` | P1 |
| `TS-binding-binding-nomsgpack-009` | `Default` | `RISK-binding-binding-nomsgpack` | P1 |
| `TS-binding-binding-nomsgpack-010` | `validate` | `RISK-binding-binding-nomsgpack` | P1 |
| `TS-binding-binding-nomsgpack-011` | `validate` | `RISK-binding-binding-nomsgpack` | P1 |
| `TS-binding-binding-nomsgpack-012` | `validate` | `RISK-binding-binding-nomsgpack` | P1 |
| `TS-binding-binding-nomsgpack-013` | `validate` | `RISK-binding-binding-nomsgpack` | P1 |
| `TS-binding-binding-nomsgpack-014` | `Default` | `RISK-binding-binding-nomsgpack` | P1 |
| `TS-binding-default-validator-001` | `ValidateStruct` | `RISK-binding-default-validator` | P1 |
| `TS-binding-default-validator-002` | `ValidateStruct` | `RISK-binding-default-validator` | P1 |
| `TS-binding-default-validator-003` | `ValidateStruct` | `RISK-binding-default-validator` | P1 |
| `TS-binding-default-validator-004` | `ValidateStruct` | `RISK-binding-default-validator` | P1 |
| `TS-binding-default-validator-005` | `ValidateStruct` | `RISK-binding-default-validator` | P1 |
| `TS-binding-default-validator-006` | `ValidateStruct` | `RISK-binding-default-validator` | P1 |
| `TS-binding-default-validator-007` | `ValidateStruct` | `RISK-binding-default-validator` | P1 |
| `TS-binding-default-validator-008` | `ValidateStruct` | `RISK-binding-default-validator` | P1 |
| `TS-binding-default-validator-009` | `ValidateStruct` | `RISK-binding-default-validator` | P1 |
| `TS-binding-default-validator-010` | `ValidateStruct` | `RISK-binding-default-validator` | P1 |
| `TS-binding-default-validator-011` | `ValidateStruct` | `RISK-binding-default-validator` | P1 |
| `TS-binding-default-validator-012` | `ValidateStruct` | `RISK-binding-default-validator` | P1 |
| `TS-binding-form-001` | `Bind` | `RISK-binding-form` | P0 |
| `TS-binding-form-002` | `Bind` | `RISK-binding-form` | P0 |
| `TS-binding-form-003` | `Bind` | `RISK-binding-form` | P0 |
| `TS-binding-form-004` | `Bind` | `RISK-binding-form` | P0 |
| `TS-binding-form-005` | `Bind` | `RISK-binding-form` | P0 |
| `TS-binding-form-mapping-001` | `MapFormWithTag` | `RISK-binding-form-mapping` | P0 |
| `TS-binding-form-mapping-002` | `MapFormWithTag` | `RISK-binding-form-mapping` | P0 |
| `TS-binding-form-mapping-003` | `MapFormWithTag` | `RISK-binding-form-mapping` | P0 |
| `TS-binding-form-mapping-004` | `MapFormWithTag` | `RISK-binding-form-mapping` | P0 |
| `TS-binding-form-mapping-005` | `MapFormWithTag` | `RISK-binding-form-mapping` | P0 |
| `TS-binding-header-001` | `Bind` | `RISK-binding-header` | P1 |
| `TS-binding-header-002` | `Bind` | `RISK-binding-header` | P1 |
| `TS-binding-header-003` | `Bind` | `RISK-binding-header` | P1 |
| `TS-binding-header-004` | `Bind` | `RISK-binding-header` | P1 |
| `TS-binding-json-001` | `Bind` | `RISK-binding-json` | P1 |
| `TS-binding-json-002` | `Bind` | `RISK-binding-json` | P1 |
| `TS-binding-json-003` | `BindBody` | `RISK-binding-json` | P1 |
| `TS-binding-json-004` | `BindBody` | `RISK-binding-json` | P1 |
| `TS-binding-json-005` | `Bind` | `RISK-binding-json` | P1 |
| `TS-binding-msgpack-001` | `Bind` | `RISK-binding-msgpack` | P1 |
| `TS-binding-msgpack-002` | `Bind` | `RISK-binding-msgpack` | P1 |
| `TS-binding-msgpack-003` | `Bind` | `RISK-binding-msgpack` | P1 |
| `TS-binding-msgpack-004` | `BindBody` | `RISK-binding-msgpack` | P1 |
| `TS-binding-msgpack-005` | `BindBody` | `RISK-binding-msgpack` | P1 |
| `TS-binding-msgpack-006` | `msgpackBinding.BindBody` | `RISK-binding-msgpack` | P1 |
| `TS-binding-multipart-form-mapping-001` | `TrySet` | `RISK-binding-multipart-form-mapping` | P1 |
| `TS-binding-multipart-form-mapping-002` | `TrySet` | `RISK-binding-multipart-form-mapping` | P1 |
| `TS-binding-multipart-form-mapping-003` | `TrySet` | `RISK-binding-multipart-form-mapping` | P1 |
| `TS-binding-multipart-form-mapping-004` | `TrySet` | `RISK-binding-multipart-form-mapping` | P1 |
| `TS-binding-plain-001` | `Bind` | `RISK-binding-plain` | P1 |
| `TS-binding-plain-002` | `Bind` | `RISK-binding-plain` | P1 |
| `TS-binding-plain-003` | `Bind` | `RISK-binding-plain` | P1 |
| `TS-binding-plain-004` | `Bind` | `RISK-binding-plain` | P1 |
| `TS-binding-plain-005` | `Bind` | `RISK-binding-plain` | P1 |
| `TS-binding-query-001` | `Bind` | `RISK-binding-query` | P1 |
| `TS-binding-query-002` | `Bind` | `RISK-binding-query` | P1 |
| `TS-binding-query-003` | `Bind` | `RISK-binding-query` | P1 |
| `TS-binding-toml-001` | `Bind` | `RISK-binding-toml` | P1 |
| `TS-binding-toml-002` | `BindBody` | `RISK-binding-toml` | P1 |
| `TS-binding-toml-003` | `Bind` | `RISK-binding-toml` | P1 |
| `TS-binding-toml-004` | `BindBody` | `RISK-binding-toml` | P1 |
| `TS-binding-toml-005` | `Bind` | `RISK-binding-toml` | P1 |
| `TS-binding-toml-006` | `BindBody` | `RISK-binding-toml` | P1 |
| `TS-binding-xml-001` | `xmlBinding.Bind` | `RISK-binding-xml` | P1 |
| `TS-binding-xml-002` | `xmlBinding.Bind` | `RISK-binding-xml` | P1 |
| `TS-binding-xml-003` | `xmlBinding.BindBody` | `RISK-binding-xml` | P1 |
| `TS-binding-xml-004` | `xmlBinding.BindBody` | `RISK-binding-xml` | P1 |
| `TS-binding-xml-005` | `xmlBinding.Bind` | `RISK-binding-xml` | P1 |
| `TS-binding-xml-006` | `xmlBinding.BindBody` | `RISK-binding-xml` | P1 |
| `TS-binding-xml-007` | `BindBody` | `RISK-binding-xml` | P1 |
| `TS-binding-yaml-001` | `yamlBinding.Bind` | `RISK-binding-yaml` | P1 |
| `TS-binding-yaml-002` | `yamlBinding.BindBody` | `RISK-binding-yaml` | P1 |
| `TS-binding-yaml-003` | `yamlBinding.Bind` | `RISK-binding-yaml` | P1 |
| `TS-binding-yaml-004` | `yamlBinding.BindBody` | `RISK-binding-yaml` | P1 |
| `TS-debug-001` | `debugPrint` | `RISK-debug` | P1 |
| `TS-debug-002` | `debugPrint` | `RISK-debug` | P1 |
| `TS-debug-003` | `debugPrint` | `RISK-debug` | P1 |
| `TS-debug-004` | `debugPrintWARNINGDefault` | `RISK-debug` | P1 |
| `TS-debug-005` | `debugPrintError` | `RISK-debug` | P1 |
| `TS-debug-006` | `debugPrintError` | `RISK-debug` | P1 |
| `TS-debug-007` | `debugPrintLoadTemplate` | `RISK-debug` | P1 |
| `TS-debug-008` | `debugPrintLoadTemplate` | `RISK-debug` | P1 |
| `TS-debug-009` | `IsDebugging` | `RISK-debug` | P1 |
| `TS-errors-001` | `Error` | `RISK-errors` | P1 |
| `TS-errors-002` | `Error.JSON` | `RISK-errors` | P1 |
| `TS-errors-003` | `Error.MarshalJSON` | `RISK-errors` | P1 |
| `TS-errors-004` | `Error.IsType` | `RISK-errors` | P1 |
| `TS-errors-005` | `errorMsgs.String` | `RISK-errors` | P1 |
| `TS-errors-006` | `errorMsgs.Errors` | `RISK-errors` | P1 |
| `TS-errors-007` | `String` | `RISK-errors` | P1 |
| `TS-errors-008` | `String` | `RISK-errors` | P1 |
| `TS-fs-001` | `OnlyFilesFS.Open` | `RISK-fs` | P1 |
| `TS-fs-002` | `OnlyFilesFS.Open` | `RISK-fs` | P1 |
| `TS-fs-003` | `OnlyFilesFS.Open` | `RISK-fs` | P1 |
| `TS-fs-004` | `neutralizedReaddirFile.Readdir` | `RISK-fs` | P1 |
| `TS-gin-001` | `SetTrustedProxies` | `RISK-gin` | P0 |
| `TS-gin-002` | `Run` | `RISK-gin` | P0 |
| `TS-gin-003` | `ServeHTTP` | `RISK-gin` | P0 |
| `TS-gin-004` | `redirectRequest` | `RISK-gin` | P0 |
| `TS-gin-005` | `prepareTrustedCIDRs` | `RISK-gin` | P0 |
| `TS-gins-gins-001` | `Run` | `RISK-gins-gins` | P1 |
| `TS-gins-gins-002` | `RunTLS` | `RISK-gins-gins` | P1 |
| `TS-gins-gins-003` | `RunUnix` | `RISK-gins-gins` | P1 |
| `TS-gins-gins-004` | `Handle` | `RISK-gins-gins` | P1 |
| `TS-gins-gins-005` | `LoadHTMLGlob` | `RISK-gins-gins` | P1 |
| `TS-gins-gins-006` | `Group` | `RISK-gins-gins` | P1 |
| `TS-gins-gins-007` | `NoMethod` | `RISK-gins-gins` | P1 |
| `TS-json-go-json-001` | `gojsonApi.Marshal` | `RISK-json-go-json` | P0 |
| `TS-json-go-json-002` | `gojsonApi.Unmarshal` | `RISK-json-go-json` | P0 |
| `TS-json-go-json-003` | `gojsonApi.MarshalIndent` | `RISK-json-go-json` | P0 |
| `TS-json-go-json-004` | `gojsonApi.NewEncoder` | `RISK-json-go-json` | P0 |
| `TS-json-go-json-005` | `gojsonApi.NewDecoder` | `RISK-json-go-json` | P0 |
| `TS-json-json-001` | `Marshal` | `RISK-json-json` | P0 |
| `TS-json-json-002` | `Unmarshal` | `RISK-json-json` | P0 |
| `TS-json-json-003` | `MarshalIndent` | `RISK-json-json` | P0 |
| `TS-json-json-004` | `NewEncoder` | `RISK-json-json` | P0 |
| `TS-json-json-005` | `NewDecoder` | `RISK-json-json` | P0 |
| `TS-json-json-006` | `NewDecoder` | `RISK-json-json` | P0 |
| `TS-json-jsoniter-001` | `jsoniterApi.Marshal` | `RISK-json-jsoniter` | P0 |
| `TS-json-jsoniter-002` | `jsoniterApi.Unmarshal` | `RISK-json-jsoniter` | P0 |
| `TS-json-jsoniter-003` | `jsoniterApi.MarshalIndent` | `RISK-json-jsoniter` | P0 |
| `TS-json-sonic-001` | `sonicApi.Marshal` | `RISK-json-sonic` | P0 |
| `TS-json-sonic-002` | `sonicApi.Unmarshal` | `RISK-json-sonic` | P0 |
| `TS-json-sonic-003` | `sonicApi.MarshalIndent` | `RISK-json-sonic` | P0 |
| `TS-json-sonic-004` | `sonicApi.NewEncoder` | `RISK-json-sonic` | P0 |
| `TS-json-sonic-005` | `sonicApi.NewDecoder` | `RISK-json-sonic` | P0 |
| `TS-logger-001` | `LoggerWithWriter` | `RISK-logger` | P1 |
| `TS-logger-002` | `LoggerWithWriter` | `RISK-logger` | P1 |
| `TS-logger-003` | `Logger` | `RISK-logger` | P1 |
| `TS-logger-004` | `Logger` | `RISK-logger` | P1 |
| `TS-logger-005` | `Logger` | `RISK-logger` | P1 |
| `TS-mode-001` | `SetMode` | `RISK-mode` | P1 |
| `TS-mode-002` | `SetMode` | `RISK-mode` | P1 |
| `TS-mode-003` | `SetMode` | `RISK-mode` | P1 |
| `TS-mode-004` | `SetMode` | `RISK-mode` | P1 |
| `TS-mode-005` | `DisableBindValidation` | `RISK-mode` | P1 |
| `TS-mode-006` | `EnableJsonDecoderUseNumber` | `RISK-mode` | P1 |
| `TS-mode-007` | `EnableJsonDecoderDisallowUnknownFields` | `RISK-mode` | P1 |
| `TS-recovery-001` | `recovery.go` | `RISK-recovery` | P1 |
| `TS-recovery-002` | `recovery.go` | `RISK-recovery` | P1 |
| `TS-recovery-003` | `recovery.go` | `RISK-recovery` | P1 |
| `TS-recovery-004` | `recovery.go` | `RISK-recovery` | P1 |
| `TS-recovery-005` | `recovery.go` | `RISK-recovery` | P1 |
| `TS-render-bson-001` | `BSON.Render` | `RISK-render-bson` | P1 |
| `TS-render-bson-002` | `BSON.Render` | `RISK-render-bson` | P1 |
| `TS-render-bson-003` | `BSON.Render` | `RISK-render-bson` | P1 |
| `TS-render-html-001` | `HTML.Render` | `RISK-render-html` | P0 |
| `TS-render-html-002` | `HTML.Render` | `RISK-render-html` | P0 |
| `TS-render-html-003` | `HTML.Render` | `RISK-render-html` | P0 |
| `TS-render-html-004` | `HTML.WriteContentType` | `RISK-render-html` | P0 |
| `TS-render-html-005` | `HTMLDebug.loadTemplate` | `RISK-render-html` | P0 |
| `TS-render-json-001` | `WriteJSON` | `RISK-render-json` | P0 |
| `TS-render-json-002` | `WriteJSON` | `RISK-render-json` | P0 |
| `TS-render-json-003` | `SecureJSON.Render` | `RISK-render-json` | P0 |
| `TS-render-json-004` | `JsonpJSON.Render` | `RISK-render-json` | P0 |
| `TS-render-json-005` | `WriteJSON` | `RISK-render-json` | P0 |
| `TS-render-json-006` | `JsonpJSON.WriteContentType` | `RISK-render-json` | P0 |
| `TS-render-json-007` | `Render` | `RISK-render-json` | P0 |
| `TS-render-json-008` | `Render` | `RISK-render-json` | P0 |
| `TS-render-json-009` | `Render` | `RISK-render-json` | P0 |
| `TS-render-json-010` | `Render` | `RISK-render-json` | P0 |
| `TS-render-msgpack-001` | `Render` | `RISK-render-msgpack` | P1 |
| `TS-render-msgpack-002` | `WriteContentType` | `RISK-render-msgpack` | P1 |
| `TS-render-msgpack-003` | `WriteMsgPack` | `RISK-render-msgpack` | P1 |
| `TS-render-msgpack-004` | `Render` | `RISK-render-msgpack` | P1 |
| `TS-render-msgpack-005` | `WriteContentType` | `RISK-render-msgpack` | P1 |
| `TS-render-protobuf-001` | `ProtoBuf.Render` | `RISK-render-protobuf` | P1 |
| `TS-render-protobuf-002` | `ProtoBuf.Render` | `RISK-render-protobuf` | P1 |
| `TS-render-protobuf-003` | `ProtoBuf.Render` | `RISK-render-protobuf` | P1 |
| `TS-render-protobuf-004` | `ProtoBuf.WriteContentType` | `RISK-render-protobuf` | P1 |
| `TS-render-reader-001` | `Reader.Render` | `RISK-render-reader` | P1 |
| `TS-render-reader-002` | `Reader.Render` | `RISK-render-reader` | P1 |
| `TS-render-reader-003` | `Reader.Render` | `RISK-render-reader` | P1 |
| `TS-render-reader-004` | `Reader.Render` | `RISK-render-reader` | P1 |
| `TS-render-reader-005` | `Reader.Render` | `RISK-render-reader` | P1 |
| `TS-render-text-001` | `Render` | `RISK-render-text` | P1 |
| `TS-render-text-002` | `Render` | `RISK-render-text` | P1 |
| `TS-render-text-003` | `Render` | `RISK-render-text` | P1 |
| `TS-render-text-004` | `WriteString` | `RISK-render-text` | P1 |
| `TS-render-text-005` | `Render` | `RISK-render-text` | P1 |
| `TS-render-xml-001` | `Render` | `RISK-render-xml` | P1 |
| `TS-render-xml-002` | `Render` | `RISK-render-xml` | P1 |
| `TS-render-xml-003` | `Render` | `RISK-render-xml` | P1 |
| `TS-render-xml-004` | `Render` | `RISK-render-xml` | P1 |
| `TS-response-writer-001` | `WriteHeader` | `RISK-response-writer` | P0 |
| `TS-response-writer-002` | `WriteHeader` | `RISK-response-writer` | P0 |
| `TS-response-writer-003` | `Hijack` | `RISK-response-writer` | P0 |
| `TS-response-writer-004` | `Hijack` | `RISK-response-writer` | P0 |
| `TS-response-writer-005` | `Flush` | `RISK-response-writer` | P0 |
| `TS-response-writer-006` | `CloseNotify` | `RISK-response-writer` | P0 |
| `TS-routergroup-001` | `RouterGroup.Handle` | `RISK-routergroup` | P0 |
| `TS-routergroup-002` | `RouterGroup.Group` | `RISK-routergroup` | P0 |
| `TS-routergroup-003` | `RouterGroup.StaticFile` | `RISK-routergroup` | P0 |
| `TS-routergroup-004` | `RouterGroup.StaticFS` | `RISK-routergroup` | P0 |
| `TS-routergroup-005` | `RouterGroup.StaticFile` | `RISK-routergroup` | P0 |
| `TS-routergroup-006` | `RouterGroup.Handle` | `RISK-routergroup` | P0 |
| `TS-test-helpers-001` | `EngineCreation` | `RISK-test-helpers` | P1 |
| `TS-test-helpers-002` | `ContextAllocation` | `RISK-test-helpers` | P1 |
| `TS-test-helpers-003` | `HTTPRequest` | `RISK-test-helpers` | P1 |
| `TS-test-helpers-004` | `ExponentialBackoff` | `RISK-test-helpers` | P1 |
| `TS-tree-001` | `addRoute` | `RISK-tree` | P1 |
| `TS-tree-002` | `addRoute` | `RISK-tree` | P1 |
| `TS-tree-003` | `addRoute` | `RISK-tree` | P1 |
| `TS-tree-004` | `addRoute` | `RISK-tree` | P1 |
| `TS-tree-005` | `addRoute` | `RISK-tree` | P1 |
| `TS-tree-006` | `addRoute` | `RISK-tree` | P1 |
| `TS-utils-001` | `Bind` | `RISK-utils` | P0 |
| `TS-utils-002` | `MarshalXML` | `RISK-utils` | P0 |
| `TS-utils-003` | `lastChar` | `RISK-utils` | P0 |
| `TS-utils-004` | `resolveAddress` | `RISK-utils` | P0 |
| `TS-utils-005` | `chooseData` | `RISK-utils` | P0 |

## Outstanding review findings

The automated reviewer raised these and they were not resolved:

- **blocking** `RISK-response-writer` — RISK-response-writer has no scenarios covering all its failure paths, and there are five distinct failure paths identified. _(Add test scenarios covering scenarios associated with all five distinct failure paths.)_
- **blocking** `RISK-binding-form` — RISK-binding-form has no scenarios covering all its failure paths, with four distinct failure paths identified. _(Add test scenarios covering scenarios associated with all four distinct failure paths.)_
- **blocking** `RISK-gin` — RISK-gin has no scenarios covering all its failure paths, with five distinct failure paths identified. _(Add test scenarios covering scenarios associated with all five distinct failure paths.)_
- **blocking** `RISK-json-go-json` — RISK-json-go-json has no scenarios covering all its failure paths, with three distinct failure paths identified. _(Add test scenarios covering scenarios associated with all three distinct failure paths.)_
- **blocking** `RISK-json-sonic` — RISK-json-sonic has no scenarios covering all its failure paths, with five distinct failure paths identified. _(Add test scenarios covering scenarios associated with all five distinct failure paths.)_
- **blocking** `RISK-json-jsoniter` — RISK-json-jsoniter has no scenarios covering all its failure paths, with three distinct failure paths identified. _(Add test scenarios covering scenarios associated with all three distinct failure paths.)_
- **blocking** `RISK-render-json` — RISK-render-json has no scenarios covering all its failure paths, with three distinct failure paths identified. _(Add test scenarios covering scenarios associated with all three distinct failure paths.)_
- **blocking** `RISK-json-json` — RISK-json-json has no scenarios covering all its failure paths, with five distinct failure paths identified. _(Add test scenarios covering scenarios associated with all five distinct failure paths.)_
- **blocking** `RISK-binding-form-mapping` — RISK-binding-form-mapping has no scenarios covering all its failure paths, with five distinct failure paths identified. _(Add test scenarios covering scenarios associated with all five distinct failure paths.)_
- **blocking** `RISK-routergroup` — RISK-routergroup has no scenarios covering all its failure paths, with four distinct failure paths identified. _(Add test scenarios covering scenarios associated with all four distinct failure paths.)_
- **blocking** `RISK-render-html` — RISK-render-html has no scenarios covering all its failure paths, with four distinct failure paths identified. _(Add test scenarios covering scenarios associated with all four distinct failure paths.)_
- **blocking** `RISK-utils` — RISK-utils has no scenarios covering all its failure paths, with five distinct failure paths identified. _(Add test scenarios covering scenarios associated with all five distinct failure paths.)_
- **blocking** `RISK-auth` — RISK-auth has no scenarios covering all its failure paths, with four distinct failure paths identified. _(Add test scenarios covering scenarios associated with all four distinct failure paths.)_
- **blocking** `RISK-logger` — RISK-logger has no scenarios covering all its failure paths, with four distinct failure paths identified. _(Add test scenarios covering scenarios associated with all four distinct failure paths.)_
- **blocking** `RISK-tree` — RISK-tree has no scenarios covering all its failure paths, with six distinct failure paths identified. _(Add test scenarios covering scenarios associated with all six distinct failure paths.)_
- **blocking** `RISK-gins-gins` — RISK-gins-gins has no scenarios covering all its failure paths, with five distinct failure paths identified. _(Add test scenarios covering scenarios associated with all five distinct failure paths.)_
- **blocking** `RISK-debug` — RISK-debug has no scenarios covering all its failure paths, with four distinct failure paths identified. _(Add test scenarios covering scenarios associated with all four distinct failure paths.)_
- **blocking** `RISK-binding-default-validator` — RISK-binding-default-validator has no scenarios covering all its failure paths, with three distinct failure paths identified. _(Add test scenarios covering scenarios associated with all three distinct failure paths.)_
- **blocking** `RISK-render-reader` — RISK-render-reader has no scenarios covering all its failure paths, with three distinct failure paths identified. _(Add test scenarios covering scenarios associated with all three distinct failure paths.)_
- **blocking** `RISK-binding-binding` — RISK-binding-binding has no scenarios covering all its failure paths, with four distinct failure paths identified. _(Add test scenarios covering scenarios associated with all four distinct failure paths.)_
- **blocking** `RISK-errors` — RISK-errors has no scenarios covering all its failure paths, with four distinct failure paths identified. _(Add test scenarios covering scenarios associated with all four distinct failure paths.)_
- **blocking** `RISK-recovery` — RISK-recovery has no scenarios covering all its failure paths, with four distinct failure paths identified. _(Add test scenarios covering scenarios associated with all four distinct failure paths.)_
- **blocking** `RISK-test-helpers` — RISK-test-helpers has no scenarios covering all its failure paths, with four distinct failure paths identified. _(Add test scenarios covering scenarios associated with all four distinct failure paths.)_
- **blocking** `RISK-binding-json` — RISK-binding-json has no scenarios covering all its failure paths, with three distinct failure paths identified. _(Add test scenarios covering scenarios associated with all three distinct failure paths.)_
- **blocking** `RISK-binding-msgpack` — RISK-binding-msgpack has no scenarios covering all its failure paths, with three distinct failure paths identified. _(Add test scenarios covering scenarios associated with all three distinct failure paths.)_
- **blocking** `RISK-binding-xml` — RISK-binding-xml has no scenarios covering all its failure paths, with three distinct failure paths identified. _(Add test scenarios covering scenarios associated with all three distinct failure paths.)_
- **blocking** `RISK-binding-binding-nomsgpack` — RISK-binding-binding-nomsgpack has no scenarios covering all its failure paths, with five distinct failure paths identified. _(Add test scenarios covering scenarios associated with all five distinct failure paths.)_
- **blocking** `RISK-binding-toml` — RISK-binding-toml has no scenarios covering all its failure paths, with two distinct failure paths identified. _(Add test scenarios covering scenarios associated with both distinct failure paths.)_
- **blocking** `RISK-binding-yaml` — RISK-binding-yaml has no scenarios covering all its failure paths, with two distinct failure paths identified. _(Add test scenarios covering scenarios associated with both distinct failure paths.)_
- **blocking** `RISK-mode` — RISK-mode has no scenarios covering all its failure paths, with two distinct failure paths identified. _(Add test scenarios covering scenarios associated with both distinct failure paths.)_
- **blocking** `RISK-render-protobuf` — RISK-render-protobuf has no scenarios covering all its failure paths, with two distinct failure paths identified. _(Add test scenarios covering scenarios associated with both distinct failure paths.)_
- **blocking** `RISK-binding-multipart-form-mapping` — RISK-binding-multipart-form-mapping has no scenarios covering all its failure paths, with three distinct failure paths identified. _(Add test scenarios covering scenarios associated with all three distinct failure paths.)_
- **blocking** `RISK-binding-plain` — RISK-binding-plain has no scenarios covering all its failure paths, with three distinct failure paths identified. _(Add test scenarios covering scenarios associated with all three distinct failure paths.)_
- **blocking** `RISK-fs` — RISK-fs has no scenarios covering all its failure paths, with three distinct failure paths identified. _(Add test scenarios covering scenarios associated with all three distinct failure paths.)_
- **blocking** `RISK-binding-header` — RISK-binding-header has no scenarios covering all its failure paths, with two distinct failure paths identified. _(Add test scenarios covering scenarios associated with both distinct failure paths.)_
- **blocking** `RISK-binding-query` — RISK-binding-query has no scenarios covering all its failure paths, with two distinct failure paths identified. _(Add test scenarios covering scenarios associated with both distinct failure paths.)_
- **blocking** `RISK-render-bson` — RISK-render-bson has no scenarios covering all its failure paths, with two distinct failure paths identified. _(Add test scenarios covering scenarios associated with both distinct failure paths.)_
- **blocking** `RISK-render-msgpack` — RISK-render-msgpack has no scenarios covering all its failure paths, with two distinct failure paths identified. _(Add test scenarios covering scenarios associated with both distinct failure)_

## Gaps

- **surveyor** — `auth_test.go`: test file did not parse: auth_test.go:1:1: expected 'package', found successfully
- **surveyor** — `benchmarks_test.go`: test file did not parse: benchmarks_test.go:1:1: expected 'package', found successfully
- **surveyor** — `binding/binding_msgpack_test.go`: test file did not parse: binding/binding_msgpack_test.go:1:1: expected 'package', found successfully
- **surveyor** — `binding/binding_test.go`: test file did not parse: binding/binding_test.go:1:1: expected 'package', found successfully
- **surveyor** — `binding/default_validator_benchmark_test.go`: test file did not parse: binding/default_validator_benchmark_test.go:1:1: expected 'package', found successfully
- **surveyor** — `binding/default_validator_test.go`: test file did not parse: binding/default_validator_test.go:1:1: expected 'package', found successfully
- **surveyor** — `binding/form_mapping_benchmark_test.go`: test file did not parse: binding/form_mapping_benchmark_test.go:1:1: expected 'package', found successfully
- **surveyor** — `binding/form_mapping_test.go`: test file did not parse: binding/form_mapping_test.go:1:1: expected 'package', found successfully
- **surveyor** — `binding/json_test.go`: test file did not parse: binding/json_test.go:1:1: expected 'package', found successfully (and 1 more errors)
- **surveyor** — `binding/msgpack_test.go`: test file did not parse: binding/msgpack_test.go:1:1: expected 'package', found successfully
- **surveyor** — `binding/multipart_form_mapping_test.go`: test file did not parse: binding/multipart_form_mapping_test.go:1:1: expected 'package', found successfully
- **surveyor** — `binding/toml_test.go`: test file did not parse: binding/toml_test.go:1:1: expected 'package', found successfully
- **surveyor** — `binding/validate_test.go`: test file did not parse: binding/validate_test.go:1:1: expected 'package', found successfully
- **surveyor** — `binding/xml_test.go`: test file did not parse: binding/xml_test.go:1:1: expected 'package', found successfully
- **surveyor** — `binding/yaml_test.go`: test file did not parse: binding/yaml_test.go:1:1: expected 'package', found successfully
- **surveyor** — `context_file_test.go`: test file did not parse: context_file_test.go:1:1: expected 'package', found successfully
- **surveyor** — `context_test.go`: test file did not parse: context_test.go:1:1: expected 'package', found successfully
- **surveyor** — `debug_test.go`: test file did not parse: debug_test.go:1:1: expected 'package', found successfully
- **surveyor** — `deprecated_test.go`: test file did not parse: deprecated_test.go:1:1: expected 'package', found successfully
- **surveyor** — `errors_test.go`: test file did not parse: errors_test.go:1:1: expected 'package', found successfully
- **surveyor** — `fs_test.go`: test file did not parse: fs_test.go:1:1: expected 'package', found successfully
- **surveyor** — `ginS/gins_test.go`: test file did not parse: ginS/gins_test.go:1:1: expected 'package', found successfully
- **surveyor** — `gin_integration_test.go`: test file did not parse: gin_integration_test.go:1:1: expected 'package', found successfully
- **surveyor** — `gin_test.go`: test file did not parse: gin_test.go:1:1: expected 'package', found successfully
- **surveyor** — `githubapi_test.go`: test file did not parse: githubapi_test.go:1:1: expected 'package', found successfully
- **surveyor** — `internal/bytesconv/bytesconv_test.go`: test file did not parse: internal/bytesconv/bytesconv_test.go:1:1: expected 'package', found successfully
- **surveyor** — `internal/fs/fs_test.go`: test file did not parse: internal/fs/fs_test.go:1:1: expected 'package', found successfully
- **surveyor** — `logger_test.go`: test file did not parse: logger_test.go:1:1: expected 'package', found successfully
- **surveyor** — `middleware_test.go`: test file did not parse: middleware_test.go:1:1: expected 'package', found successfully
- **surveyor** — `mode_test.go`: test file did not parse: mode_test.go:1:1: expected 'package', found successfully
- **surveyor** — `path_test.go`: test file did not parse: path_test.go:1:1: expected 'package', found successfully
- **surveyor** — `recovery_test.go`: test file did not parse: recovery_test.go:1:1: expected 'package', found successfully
- **surveyor** — `render/reader_test.go`: test file did not parse: render/reader_test.go:1:1: expected 'package', found successfully
- **surveyor** — `render/render_msgpack_test.go`: test file did not parse: render/render_msgpack_test.go:1:1: expected 'package', found successfully
- **surveyor** — `render/render_test.go`: test file did not parse: render/render_test.go:1:1: expected 'package', found successfully
- **surveyor** — `response_writer_test.go`: test file did not parse: response_writer_test.go:1:1: expected 'package', found successfully (and 1 more errors)
- **surveyor** — `routergroup_test.go`: test file did not parse: routergroup_test.go:1:1: expected 'package', found successfully
- **surveyor** — `routes_test.go`: test file did not parse: routes_test.go:1:1: expected 'package', found successfully
- **surveyor** — `tree_test.go`: test file did not parse: tree_test.go:1:1: expected 'package', found successfully
- **surveyor** — `utils_test.go`: test file did not parse: utils_test.go:1:1: expected 'package', found successfully
- **analyst** — `context.go`: analysis stopped early (budget_exhausted): analyst:context.go used 81108 tokens of 60000
- **analyst** — `context.go`: the analyst finished without recording an analysis (the model may not support tool calling)
- **critic** — `revision`: stopped after round 2: blocking findings did not decrease (1 then 38)
- **approval** — `verdict`: awaiting human approval; no scenario has been published
- **degraded tool** — `code_parse_go`: "version.go" did not parse as Go (version.go:1:1: expected 'package', found successfully); it will be skipped
- **degraded tool** — `repo_read_file`: could not read "test_helpers/test_helpers.go"; continue without it

