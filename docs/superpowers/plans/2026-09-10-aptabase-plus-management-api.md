# aptabase-plus Management API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fork `aptabase/aptabase` into `nmehlei/aptabase-plus` and add a
stable, versioned, API-key-authenticated management API (`/api/v0/apps`,
`/api/v0/api-keys`) so non-interactive tooling — starting with
`terraform-provider-aptabase` — can manage apps and shares without a
browser session.

**Architecture:** Add a new `api_keys` table and an
`ApiKeyAuthenticationHandler` registered as an ASP.NET Core authentication
scheme alongside the existing cookie scheme, selected per-request via a
policy scheme based on the presence of an `Authorization: Bearer` header.
Because every existing `[IsAuthenticated]`-gated endpoint authenticates via
`HttpContext.User`, this makes API keys work transparently everywhere that
attribute is used — no changes needed to existing controllers. New
controllers (`ApiKeysController`, `AppsV0Controller`) expose the stable
`/api/v0` management surface; existing internal `/api/_apps` routes and
the existing `/api/v0/apps/{appId}/errors` (cookie-only) routes are left
untouched.

**Tech Stack:** ASP.NET Core (net10.0), Dapper, FluentMigrator, Postgres,
xUnit + AwesomeAssertions (existing repo stack — no new dependencies).

**Spec:** `docs/superpowers/specs/2026-09-10-aptabase-plus-and-terraform-provider-design.md`

## Global Constraints

- Fork target: `nmehlei/aptabase-plus`, public, forked from
  `aptabase/aptabase` (current default branch `main`).
- One API key type only: user-scoped, inherits the full permissions of
  its owning user (including minting/revoking further keys for that user).
- New routes live under `/api/v0/apps` and `/api/v0/api-keys` — do not
  modify the existing `/api/_apps` controller or the existing
  `/api/v0/apps/{appId}/errors*` routes in `ErrorsController`.
- Key format: `aptb_` + 32 random bytes, base64url-alphabet, no padding.
  Stored as SHA-256 hex hash + a 12-character display prefix. Plaintext is
  returned to the caller exactly once, at creation.
- Follow existing code style exactly: inline Dapper SQL in controllers
  (not a repository/service layer), full literal route paths on each
  `[Http*]` attribute (not a class-level `[Route]`), `NanoId.New()` for
  IDs, `AwesomeAssertions` for test assertions.
- No frontend test runner exists in this repo today — the Task 8 (UI)
  deliverable is verified manually via `npm run dev`, not an automated
  test.

---

## Task 1: Fork and bootstrap the repository

**Files:**
- Modify: `README.md` (top of file)

- [ ] **Step 1: Fork the upstream repository**

This creates a public repository under the user's GitHub account —
confirm with the user before running if not already explicitly approved
for this session.

```bash
gh repo fork aptabase/aptabase --fork-name aptabase-plus --clone=true --remote=true
```

This clones to `./aptabase-plus` with `origin` pointing at
`nmehlei/aptabase-plus` and `upstream` pointing at `aptabase/aptabase`.
Move the clone to `~/Dev/aptabase-plus` if it didn't land there.

- [ ] **Step 2: Verify remotes**

```bash
git -C ~/Dev/aptabase-plus remote -v
```

Expected: `origin` → `nmehlei/aptabase-plus`, `upstream` → `aptabase/aptabase`.

- [ ] **Step 3: Add a positioning note to the top of README.md**

Insert immediately after the first heading line of
`~/Dev/aptabase-plus/README.md` (do not remove any existing content):

```markdown
> **This is `aptabase-plus`**, a downstream distribution of
> [aptabase/aptabase](https://github.com/aptabase/aptabase) maintained by
> [@nmehlei](https://github.com/nmehlei). It tracks upstream `main` and
> adds a stable, API-key-authenticated management API
> (`/api/v0/apps`, `/api/v0/api-keys`) so tools like
> [terraform-provider-aptabase](https://github.com/nmehlei/terraform-provider-aptabase)
> can manage apps without a browser session. Not officially affiliated
> with Aptabase. See `docs/upstream-proposal.md` for the design and its
> upstream discussion status.
```

- [ ] **Step 4: Commit**

```bash
git -C ~/Dev/aptabase-plus add README.md
git -C ~/Dev/aptabase-plus commit -m "docs: note aptabase-plus positioning as a downstream distribution"
```

---

## Task 2: `api_keys` table migration

**Files:**
- Create: `src/Data/Migrations/0014_AddApiKeys.cs`
- Test: verified via Task 4's integration tests (this repo has no
  per-migration unit tests; migrations run automatically on app startup)

**Interfaces:**
- Produces: table `api_keys(id, user_id, name, key_hash, key_prefix, last_used_at, expires_at, created_at, modified_at)`

- [ ] **Step 1: Write the migration**

```csharp
using FluentMigrator;

namespace Aptabase.Data.Migrations;

[Migration(0014)]
public class AddApiKeys : Migration
{
    public override void Up()
    {
        Create.Table("api_keys")
            .WithNanoIdColumn("id").PrimaryKey()
            .WithColumn("user_id").AsString(22).NotNullable().ForeignKey("users", "id")
            .WithColumn("name").AsString(100).NotNullable()
            .WithColumn("key_hash").AsString(64).NotNullable().Unique()
            .WithColumn("key_prefix").AsString(12).NotNullable()
            .WithColumn("last_used_at").AsDateTimeOffset().Nullable()
            .WithColumn("expires_at").AsDateTimeOffset().Nullable()
            .WithTimestamps();

        Create.Index("ix_api_keys_user_id")
            .OnTable("api_keys")
            .OnColumn("user_id");
    }

    public override void Down()
    {
        Delete.Table("api_keys");
    }
}
```

- [ ] **Step 2: Verify it builds**

```bash
cd ~/Dev/aptabase-plus/src && dotnet build
```

Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git -C ~/Dev/aptabase-plus add src/Data/Migrations/0014_AddApiKeys.cs
git -C ~/Dev/aptabase-plus commit -m "feat: add api_keys table migration"
```

(This migration is exercised for real by Task 4's integration test, which
starts a real Postgres via the existing `docker-compose.yml` and runs
`RunMigrations` through `CustomWebApplicationFactory`.)

---

## Task 3: API key generation and hashing

**Files:**
- Create: `src/Features/Authentication/ApiKeys/ApiKeyGenerator.cs`
- Test: `tests/UnitTests/Features/Authentication/ApiKeys/ApiKeyGeneratorTests.cs`

**Interfaces:**
- Produces: `ApiKeyGenerator.Prefix` (`const string`), `ApiKeyGenerator.Generate() -> (string PlainText, string Hash, string DisplayPrefix)`, `ApiKeyGenerator.Hash(string plainTextKey) -> string`

- [ ] **Step 1: Write the failing test**

```csharp
using Aptabase.Features.Authentication.ApiKeys;
using AwesomeAssertions;
using Xunit;

namespace Aptabase.UnitTests.Features.Authentication.ApiKeys;

public class ApiKeyGeneratorTests
{
    [Fact]
    public void Generate_ProducesUniqueKeysWithExpectedPrefix()
    {
        var (plainTextA, hashA, displayPrefixA) = ApiKeyGenerator.Generate();
        var (plainTextB, hashB, _) = ApiKeyGenerator.Generate();

        plainTextA.Should().StartWith(ApiKeyGenerator.Prefix);
        displayPrefixA.Should().Be(plainTextA[..12]);
        plainTextA.Should().NotBe(plainTextB);
        hashA.Should().NotBe(hashB);
    }

    [Fact]
    public void Hash_IsDeterministicSha256Hex()
    {
        var hash1 = ApiKeyGenerator.Hash("aptb_sample");
        var hash2 = ApiKeyGenerator.Hash("aptb_sample");

        hash1.Should().Be(hash2);
        hash1.Should().HaveLength(64);
        hash1.Should().MatchRegex("^[0-9a-f]{64}$");
    }

    [Fact]
    public void Generate_HashMatchesHashOfPlainText()
    {
        var (plainText, hash, _) = ApiKeyGenerator.Generate();

        ApiKeyGenerator.Hash(plainText).Should().Be(hash);
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd ~/Dev/aptabase-plus && dotnet test tests/UnitTests --filter ApiKeyGeneratorTests
```

Expected: FAIL — `ApiKeyGenerator` does not exist.

- [ ] **Step 3: Write the implementation**

```csharp
using System.Security.Cryptography;
using System.Text;

namespace Aptabase.Features.Authentication.ApiKeys;

public static class ApiKeyGenerator
{
    public const string Prefix = "aptb_";
    public const int DisplayPrefixLength = 12;
    private const int SecretBytes = 32;

    public static (string PlainText, string Hash, string DisplayPrefix) Generate()
    {
        var bytes = RandomNumberGenerator.GetBytes(SecretBytes);
        var secret = Convert.ToBase64String(bytes)
            .Replace("+", "")
            .Replace("/", "")
            .Replace("=", "");
        var plainText = $"{Prefix}{secret}";
        var hash = Hash(plainText);
        var displayPrefix = plainText[..Math.Min(DisplayPrefixLength, plainText.Length)];
        return (plainText, hash, displayPrefix);
    }

    public static string Hash(string plainTextKey)
    {
        var bytes = SHA256.HashData(Encoding.UTF8.GetBytes(plainTextKey));
        return Convert.ToHexString(bytes).ToLowerInvariant();
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd ~/Dev/aptabase-plus && dotnet test tests/UnitTests --filter ApiKeyGeneratorTests
```

Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git -C ~/Dev/aptabase-plus add src/Features/Authentication/ApiKeys/ApiKeyGenerator.cs tests/UnitTests/Features/Authentication/ApiKeys/ApiKeyGeneratorTests.cs
git -C ~/Dev/aptabase-plus commit -m "feat: add API key generation and hashing"
```

---

## Task 4: API key authentication scheme

**Files:**
- Create: `src/Features/Authentication/ApiKeys/ApiKeyAuthenticationHandler.cs`
- Modify: `src/Program.cs:81-91` (authentication registration)
- Modify: `tests/IntegrationTests/IntegrationTestsFixture.cs` (add a generic service accessor)
- Modify: `tests/IntegrationTests/Clients/AccountClient.cs` (add `GetMeAsync`)
- Test: `tests/IntegrationTests/ApiKeyAuthenticationHandlerTests.cs`

**Interfaces:**
- Consumes: `ApiKeyGenerator.Generate()`, `ApiKeyGenerator.Hash(string)` (Task 3); `IDbContext` (existing); `NanoId.New()` (existing)
- Produces: `ApiKeyAuthenticationHandler.SchemeName` (`const string`, value `"ApiKey"`); every `[IsAuthenticated]` endpoint now also accepts `Authorization: Bearer <key>`

- [ ] **Step 1: Add `GetService<T>` to the test fixture**

In `tests/IntegrationTests/IntegrationTestsFixture.cs`, add next to the
existing `GetHostedService<T>` method:

```csharp
public T GetService<T>() where T : notnull
{
    using var scope = _factory.Services.CreateScope();
    return scope.ServiceProvider.GetRequiredService<T>();
}
```

- [ ] **Step 2: Add `GetMeAsync` to `AccountClient`**

In `tests/IntegrationTests/Clients/AccountClient.cs`, add:

```csharp
public async Task<UserAccount?> GetMeAsync()
{
    return await _client.GetFromJsonAsync<UserAccount>("/api/_auth/me");
}
```

Add `using Aptabase.Features.Authentication;` to that file's usings if not
already present.

- [ ] **Step 3: Write the failing integration test**

```csharp
using System.Net;
using System.Net.Http.Headers;
using Aptabase.Data;
using Aptabase.Features.Authentication.ApiKeys;
using AwesomeAssertions;
using Dapper;
using Xunit;

namespace Aptabase.IntegrationTests;

[Collection("Integration Tests")]
public class ApiKeyAuthenticationHandlerTests
{
    private readonly IntegrationTestsFixture _fixture;

    public ApiKeyAuthenticationHandlerTests(IntegrationTestsFixture fixture)
    {
        _fixture = fixture;
    }

    [Fact]
    public async Task ValidApiKey_AuthenticatesAsKeyOwner()
    {
        var me = await _fixture.UserA.GetMeAsync();
        var plainTextKey = await InsertApiKeyForUserAsync(me!.Id);

        var client = _fixture.CreateClient();
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", plainTextKey);

        var response = await client.GetAsync("/api/_auth/me");

        response.StatusCode.Should().Be(HttpStatusCode.OK);
        var body = await response.Content.ReadFromJsonAsync<Aptabase.Features.Authentication.UserAccount>();
        body!.Id.Should().Be(me.Id);
    }

    [Fact]
    public async Task UnknownApiKey_ReturnsUnauthorized()
    {
        var client = _fixture.CreateClient();
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", "aptb_doesnotexist");

        var response = await client.GetAsync("/api/_auth/me");

        response.StatusCode.Should().Be(HttpStatusCode.Unauthorized);
    }

    [Fact]
    public async Task ExpiredApiKey_ReturnsUnauthorized()
    {
        var me = await _fixture.UserA.GetMeAsync();
        var plainTextKey = await InsertApiKeyForUserAsync(me!.Id, expiresAt: DateTimeOffset.UtcNow.AddMinutes(-1));

        var client = _fixture.CreateClient();
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", plainTextKey);

        var response = await client.GetAsync("/api/_auth/me");

        response.StatusCode.Should().Be(HttpStatusCode.Unauthorized);
    }

    private async Task<string> InsertApiKeyForUserAsync(string userId, DateTimeOffset? expiresAt = null)
    {
        var db = _fixture.GetService<IDbContext>();
        var (plainText, hash, displayPrefix) = ApiKeyGenerator.Generate();
        var id = NanoId.New();

        await db.Connection.ExecuteAsync(
            @"INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, expires_at)
              VALUES (@id, @userId, 'test key', @hash, @displayPrefix, @expiresAt)",
            new { id, userId, hash, displayPrefix, expiresAt });

        return plainText;
    }
}
```

- [ ] **Step 4: Run test to verify it fails**

```bash
cd ~/Dev/aptabase-plus
docker compose up -d
dotnet test tests/IntegrationTests --filter ApiKeyAuthenticationHandlerTests
```

Expected: FAIL — `ApiKeyAuthenticationHandler`/`ApiKeyGenerator` referenced
type resolves (from Task 3) but every request returns 200/401 from the
cookie-only path regardless of the `Authorization` header (no handler
wired in yet), so `ValidApiKey_AuthenticatesAsKeyOwner` fails with 401.

- [ ] **Step 5: Write the authentication handler**

```csharp
using System.Security.Claims;
using System.Text.Encodings.Web;
using Aptabase.Data;
using Dapper;
using Microsoft.AspNetCore.Authentication;
using Microsoft.Extensions.Options;

namespace Aptabase.Features.Authentication.ApiKeys;

public class ApiKeyAuthenticationHandler : AuthenticationHandler<AuthenticationSchemeOptions>
{
    public const string SchemeName = "ApiKey";

    private readonly IDbContext _db;

    public ApiKeyAuthenticationHandler(
        IOptionsMonitor<AuthenticationSchemeOptions> options,
        ILoggerFactory logger,
        UrlEncoder encoder,
        IDbContext db)
        : base(options, logger, encoder)
    {
        _db = db ?? throw new ArgumentNullException(nameof(db));
    }

    protected override async Task<AuthenticateResult> HandleAuthenticateAsync()
    {
        var header = Request.Headers.Authorization.FirstOrDefault();
        if (string.IsNullOrEmpty(header) || !header.StartsWith("Bearer ", StringComparison.OrdinalIgnoreCase))
            return AuthenticateResult.NoResult();

        var rawKey = header["Bearer ".Length..].Trim();
        if (!rawKey.StartsWith(ApiKeyGenerator.Prefix, StringComparison.Ordinal))
            return AuthenticateResult.Fail("Malformed API key");

        var hash = ApiKeyGenerator.Hash(rawKey);

        var row = await _db.Connection.QueryFirstOrDefaultAsync<ApiKeyAuthRow>(
            @"SELECT k.id as key_id, u.id, u.name, u.email
              FROM api_keys k
              INNER JOIN users u ON u.id = k.user_id
              WHERE k.key_hash = @hash
              AND (k.expires_at IS NULL OR k.expires_at > now())",
            new { hash });

        if (row is null)
            return AuthenticateResult.Fail("Invalid, revoked, or expired API key");

        UpdateLastUsedBestEffort(row.KeyId);

        var claims = new[]
        {
            new Claim("id", row.Id),
            new Claim("name", row.Name),
            new Claim("email", row.Email),
        };
        var identity = new ClaimsIdentity(claims, SchemeName);
        var principal = new ClaimsPrincipal(identity);
        var ticket = new AuthenticationTicket(principal, SchemeName);
        return AuthenticateResult.Success(ticket);
    }

    private void UpdateLastUsedBestEffort(string keyId)
    {
        _ = Task.Run(async () =>
        {
            try
            {
                await _db.Connection.ExecuteAsync(
                    "UPDATE api_keys SET last_used_at = now() WHERE id = @id",
                    new { id = keyId });
            }
            catch (Exception ex)
            {
                Logger.LogWarning(ex, "Failed to update api_keys.last_used_at for {KeyId}", keyId);
            }
        });
    }
}

internal class ApiKeyAuthRow
{
    public string KeyId { get; set; } = "";
    public string Id { get; set; } = "";
    public string Name { get; set; } = "";
    public string Email { get; set; } = "";
}
```

- [ ] **Step 6: Wire the scheme into `Program.cs`**

Replace lines 81-91 of `src/Program.cs`:

```csharp
        builder.Services.AddAuthentication(CookieAuthenticationDefaults.AuthenticationScheme)
                        .AddCookie(options =>
                        {
                            options.ExpireTimeSpan = TimeSpan.FromDays(365);
                            options.Cookie.Name = "auth-session";
                            options.Cookie.SameSite = SameSiteMode.Strict;
                            options.Cookie.HttpOnly = true;
                            options.Cookie.IsEssential = true;
                            options.Cookie.SecurePolicy = CookieSecurePolicy.SameAsRequest;
                            options.Cookie.MaxAge = TimeSpan.FromDays(365);
                        }).AddGitHub(appEnv).AddGoogle(appEnv);
```

with:

```csharp
        const string cookieOrApiKeyScheme = "CookieOrApiKey";

        builder.Services.AddAuthentication(cookieOrApiKeyScheme)
                        .AddPolicyScheme(cookieOrApiKeyScheme, "Cookie or API Key", policyOptions =>
                        {
                            policyOptions.ForwardDefaultSelector = context =>
                            {
                                var authHeader = context.Request.Headers.Authorization.FirstOrDefault();
                                return !string.IsNullOrEmpty(authHeader) &&
                                       authHeader.StartsWith("Bearer ", StringComparison.OrdinalIgnoreCase)
                                    ? Aptabase.Features.Authentication.ApiKeys.ApiKeyAuthenticationHandler.SchemeName
                                    : CookieAuthenticationDefaults.AuthenticationScheme;
                            };
                        })
                        .AddCookie(CookieAuthenticationDefaults.AuthenticationScheme, options =>
                        {
                            options.ExpireTimeSpan = TimeSpan.FromDays(365);
                            options.Cookie.Name = "auth-session";
                            options.Cookie.SameSite = SameSiteMode.Strict;
                            options.Cookie.HttpOnly = true;
                            options.Cookie.IsEssential = true;
                            options.Cookie.SecurePolicy = CookieSecurePolicy.SameAsRequest;
                            options.Cookie.MaxAge = TimeSpan.FromDays(365);
                        })
                        .AddScheme<AuthenticationSchemeOptions, Aptabase.Features.Authentication.ApiKeys.ApiKeyAuthenticationHandler>(
                            Aptabase.Features.Authentication.ApiKeys.ApiKeyAuthenticationHandler.SchemeName, null)
                        .AddGitHub(appEnv).AddGoogle(appEnv);
```

Add `using Aptabase.Features.Authentication.ApiKeys;` to `Program.cs`'s
usings and drop the fully-qualified names above if preferred — either
compiles.

- [ ] **Step 7: Run test to verify it passes**

```bash
cd ~/Dev/aptabase-plus && dotnet test tests/IntegrationTests --filter ApiKeyAuthenticationHandlerTests
```

Expected: PASS (3 tests). Also re-run the full suite to confirm the
scheme change didn't break existing cookie-based auth:

```bash
dotnet test tests/IntegrationTests
```

Expected: all pre-existing tests still PASS.

- [ ] **Step 8: Commit**

```bash
git -C ~/Dev/aptabase-plus add src/Features/Authentication/ApiKeys/ApiKeyAuthenticationHandler.cs src/Program.cs tests/IntegrationTests/IntegrationTestsFixture.cs tests/IntegrationTests/Clients/AccountClient.cs tests/IntegrationTests/ApiKeyAuthenticationHandlerTests.cs
git -C ~/Dev/aptabase-plus commit -m "feat: authenticate requests via API key alongside cookie session"
```

---

## Task 5: API key management endpoints

**Files:**
- Create: `src/Features/Authentication/ApiKeys/ApiKeysController.cs`
- Modify: `tests/IntegrationTests/Clients/AccountClient.cs` (add key-management helpers)
- Test: `tests/IntegrationTests/ApiKeysTests.cs`

**Interfaces:**
- Consumes: `ApiKeyGenerator.Generate()` (Task 3); `IsAuthenticatedAttribute`, `GetCurrentUserIdentity()` (existing)
- Produces: `POST /api/v0/api-keys`, `GET /api/v0/api-keys`, `DELETE /api/v0/api-keys/{keyId}`

- [ ] **Step 1: Add key-management helpers to `AccountClient`**

```csharp
public async Task<ApiKeyCreated> CreateApiKeyAsync(string name, DateTimeOffset? expiresAt = null)
{
    var response = await _client.PostAsJsonAsync("/api/v0/api-keys", new { name, expiresAt });
    response.StatusCode.Should().Be(HttpStatusCode.OK);
    return (await response.Content.ReadFromJsonAsync<ApiKeyCreated>())!;
}

public async Task<ApiKeySummary[]> ListApiKeysAsync()
{
    return (await _client.GetFromJsonAsync<ApiKeySummary[]>("/api/v0/api-keys"))!;
}

public async Task<HttpResponseMessage> DeleteApiKeyAsync(string keyId)
{
    return await _client.DeleteAsync($"/api/v0/api-keys/{keyId}");
}
```

Add `using Aptabase.Features.Authentication.ApiKeys;` to
`AccountClient.cs`.

- [ ] **Step 2: Write the failing test**

```csharp
using System.Net;
using System.Net.Http.Headers;
using AwesomeAssertions;
using Xunit;

namespace Aptabase.IntegrationTests;

[Collection("Integration Tests")]
public class ApiKeysTests
{
    private readonly IntegrationTestsFixture _fixture;

    public ApiKeysTests(IntegrationTestsFixture fixture)
    {
        _fixture = fixture;
    }

    [Fact]
    public async Task CreateListAndUseApiKey_FullLifecycle()
    {
        var created = await _fixture.UserA.CreateApiKeyAsync("ci key");
        created.Key.Should().StartWith("aptb_");
        created.KeyPrefix.Should().Be(created.Key[..12]);

        var list = await _fixture.UserA.ListApiKeysAsync();
        list.Should().ContainSingle(k => k.Id == created.Id);
        list.Should().OnlyContain(k => k.KeyPrefix.Length == 12);

        var keyClient = _fixture.CreateClient();
        keyClient.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", created.Key);
        var meResponse = await keyClient.GetAsync("/api/_auth/me");
        meResponse.StatusCode.Should().Be(HttpStatusCode.OK);

        var deleteResponse = await _fixture.UserA.DeleteApiKeyAsync(created.Id);
        deleteResponse.StatusCode.Should().Be(HttpStatusCode.OK);

        meResponse = await keyClient.GetAsync("/api/_auth/me");
        meResponse.StatusCode.Should().Be(HttpStatusCode.Unauthorized);
    }

    [Fact]
    public async Task DeleteApiKey_OwnedByAnotherUser_ReturnsNotFound()
    {
        var created = await _fixture.UserA.CreateApiKeyAsync("user a key");

        var response = await _fixture.UserB.DeleteApiKeyAsync(created.Id);

        response.StatusCode.Should().Be(HttpStatusCode.NotFound);

        var stillListed = await _fixture.UserA.ListApiKeysAsync();
        stillListed.Should().Contain(k => k.Id == created.Id);
    }
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd ~/Dev/aptabase-plus && dotnet test tests/IntegrationTests --filter ApiKeysTests
```

Expected: FAIL — 404 for `/api/v0/api-keys` (no controller yet).

- [ ] **Step 4: Write the controller**

```csharp
using System.ComponentModel.DataAnnotations;
using Aptabase.Data;
using Dapper;
using Microsoft.AspNetCore.Mvc;

namespace Aptabase.Features.Authentication.ApiKeys;

public class CreateApiKeyRequestBody
{
    [Required]
    [StringLength(100, MinimumLength = 2)]
    public string Name { get; set; } = "";

    public DateTimeOffset? ExpiresAt { get; set; }
}

public class ApiKeySummary
{
    public string Id { get; set; } = "";
    public string Name { get; set; } = "";
    public string KeyPrefix { get; set; } = "";
    public DateTimeOffset? LastUsedAt { get; set; }
    public DateTimeOffset? ExpiresAt { get; set; }
    public DateTimeOffset CreatedAt { get; set; }
}

public class ApiKeyCreated : ApiKeySummary
{
    public string Key { get; set; } = "";
}

[ApiController, IsAuthenticated]
[ResponseCache(NoStore = true, Location = ResponseCacheLocation.None)]
public class ApiKeysController : Controller
{
    private readonly IDbContext _db;

    public ApiKeysController(IDbContext db)
    {
        _db = db ?? throw new ArgumentNullException(nameof(db));
    }

    [HttpGet("/api/v0/api-keys")]
    public async Task<IActionResult> List()
    {
        var user = this.GetCurrentUserIdentity();
        var keys = await _db.Connection.QueryAsync<ApiKeySummary>(
            @"SELECT id, name, key_prefix, last_used_at, expires_at, created_at
              FROM api_keys
              WHERE user_id = @userId
              ORDER BY created_at DESC",
            new { userId = user.Id });

        return Ok(keys);
    }

    [HttpPost("/api/v0/api-keys")]
    public async Task<IActionResult> Create([FromBody] CreateApiKeyRequestBody body)
    {
        var user = this.GetCurrentUserIdentity();
        var (plainText, hash, displayPrefix) = ApiKeyGenerator.Generate();
        var id = NanoId.New();
        var createdAt = DateTimeOffset.UtcNow;

        await _db.Connection.ExecuteAsync(
            @"INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, expires_at)
              VALUES (@id, @userId, @name, @hash, @displayPrefix, @expiresAt)",
            new { id, userId = user.Id, name = body.Name, hash, displayPrefix, expiresAt = body.ExpiresAt });

        return Ok(new ApiKeyCreated
        {
            Id = id,
            Name = body.Name,
            KeyPrefix = displayPrefix,
            ExpiresAt = body.ExpiresAt,
            CreatedAt = createdAt,
            Key = plainText,
        });
    }

    [HttpDelete("/api/v0/api-keys/{keyId}")]
    public async Task<IActionResult> Delete(string keyId)
    {
        var user = this.GetCurrentUserIdentity();
        var affected = await _db.Connection.ExecuteAsync(
            "DELETE FROM api_keys WHERE id = @keyId AND user_id = @userId",
            new { keyId, userId = user.Id });

        if (affected == 0)
            return NotFound();

        return Ok(new { });
    }
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
cd ~/Dev/aptabase-plus && dotnet test tests/IntegrationTests --filter ApiKeysTests
```

Expected: PASS (2 tests).

- [ ] **Step 6: Commit**

```bash
git -C ~/Dev/aptabase-plus add src/Features/Authentication/ApiKeys/ApiKeysController.cs tests/IntegrationTests/Clients/AccountClient.cs tests/IntegrationTests/ApiKeysTests.cs
git -C ~/Dev/aptabase-plus commit -m "feat: add /api/v0/api-keys management endpoints"
```

---

## Task 6: `/api/v0/apps` management endpoints

**Files:**
- Create: `src/Features/Apps/AppsV0Controller.cs`
- Modify: `tests/IntegrationTests/Clients/AccountClient.cs` (add v0 app helpers)
- Test: `tests/IntegrationTests/AppsV0Tests.cs`

**Interfaces:**
- Consumes: `ApiKeyGenerator`/API-key auth (Tasks 3-4); existing `Application` model and `ApplicationShare` (defined in `src/Features/Apps/AppsController.cs`); existing `IBlobService`, `EnvSettings`
- Produces: `GET/POST /api/v0/apps`, `GET/PUT/DELETE /api/v0/apps/{appId}`, `GET/PUT/DELETE /api/v0/apps/{appId}/shares[/{email}]`

- [ ] **Step 1: Add v0 app helpers to `AccountClient`**

```csharp
public HttpClient AuthenticatedWith(string apiKey)
{
    var client = _client;
    client.DefaultRequestHeaders.Authorization =
        new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", apiKey);
    return client;
}
```

(Reuses the same `HttpClient` the cookie session lives on is fine here
since `Authorization` and cookie can coexist; the policy scheme picks
`Authorization` first when both are present — see Task 4, Step 6.)

- [ ] **Step 2: Write the failing test**

```csharp
using System.Net;
using System.Net.Http.Json;
using AwesomeAssertions;
using Xunit;

namespace Aptabase.IntegrationTests;

[Collection("Integration Tests")]
public class AppsV0Tests
{
    private readonly IntegrationTestsFixture _fixture;

    public AppsV0Tests(IntegrationTestsFixture fixture)
    {
        _fixture = fixture;
    }

    [Fact]
    public async Task FullAppLifecycle_ViaApiKey()
    {
        var key = await _fixture.UserA.CreateApiKeyAsync("terraform");
        var client = _fixture.UserA.AuthenticatedWith(key.Key);

        var createResponse = await client.PostAsJsonAsync("/api/v0/apps", new { name = "My App" });
        createResponse.StatusCode.Should().Be(HttpStatusCode.OK);
        var created = (await createResponse.Content.ReadFromJsonAsync<AppV0>())!;
        created.Name.Should().Be("My App");
        created.AppKey.Should().NotBeNullOrEmpty();

        var getResponse = await client.GetAsync($"/api/v0/apps/{created.Id}");
        getResponse.StatusCode.Should().Be(HttpStatusCode.OK);

        var updateResponse = await client.PutAsJsonAsync($"/api/v0/apps/{created.Id}", new { name = "Renamed App", icon = "" });
        updateResponse.StatusCode.Should().Be(HttpStatusCode.OK);
        var updated = (await updateResponse.Content.ReadFromJsonAsync<AppV0>())!;
        updated.Name.Should().Be("Renamed App");

        var shareResponse = await client.PutAsync($"/api/v0/apps/{created.Id}/shares/friend@example.com", null);
        shareResponse.StatusCode.Should().Be(HttpStatusCode.OK);

        var sharesResponse = await client.GetFromJsonAsync<ShareV0[]>($"/api/v0/apps/{created.Id}/shares");
        sharesResponse.Should().ContainSingle(s => s.Email == "friend@example.com");

        var unshareResponse = await client.DeleteAsync($"/api/v0/apps/{created.Id}/shares/friend@example.com");
        unshareResponse.StatusCode.Should().Be(HttpStatusCode.OK);

        var deleteResponse = await client.DeleteAsync($"/api/v0/apps/{created.Id}");
        deleteResponse.StatusCode.Should().Be(HttpStatusCode.OK);

        getResponse = await client.GetAsync($"/api/v0/apps/{created.Id}");
        getResponse.StatusCode.Should().Be(HttpStatusCode.NotFound);
    }

    [Fact]
    public async Task GetApp_OwnedByAnotherUser_ReturnsNotFound()
    {
        var keyA = await _fixture.UserA.CreateApiKeyAsync("k");
        var clientA = _fixture.UserA.AuthenticatedWith(keyA.Key);
        var createResponse = await clientA.PostAsJsonAsync("/api/v0/apps", new { name = "Private App" });
        var created = (await createResponse.Content.ReadFromJsonAsync<AppV0>())!;

        var keyB = await _fixture.UserB.CreateApiKeyAsync("k");
        var clientB = _fixture.UserB.AuthenticatedWith(keyB.Key);
        var response = await clientB.GetAsync($"/api/v0/apps/{created.Id}");

        response.StatusCode.Should().Be(HttpStatusCode.NotFound);
    }

    private record AppV0(string Id, string Name, string AppKey);
    private record ShareV0(string Email);
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd ~/Dev/aptabase-plus && dotnet test tests/IntegrationTests --filter AppsV0Tests
```

Expected: FAIL — 404 for `/api/v0/apps` (no controller yet).

- [ ] **Step 4: Write the controller**

```csharp
using Aptabase.Data;
using Aptabase.Features.Authentication;
using Aptabase.Features.Blob;
using Dapper;
using Microsoft.AspNetCore.Mvc;
using System.ComponentModel.DataAnnotations;

namespace Aptabase.Features.Apps;

public class CreateAppV0RequestBody
{
    [Required]
    [StringLength(40, MinimumLength = 2)]
    public string Name { get; set; } = "";
}

public class UpdateAppV0RequestBody
{
    public string Icon { get; set; } = "";

    [Required]
    [StringLength(40, MinimumLength = 2)]
    public string Name { get; set; } = "";
}

[ApiController, IsAuthenticated]
[ResponseCache(NoStore = true, Location = ResponseCacheLocation.None)]
public class AppsV0Controller : Controller
{
    private readonly IDbContext _db;
    private readonly EnvSettings _env;
    private readonly IBlobService _blobService;

    public AppsV0Controller(IDbContext db, EnvSettings env, IBlobService blobService)
    {
        _db = db ?? throw new ArgumentNullException(nameof(db));
        _env = env ?? throw new ArgumentNullException(nameof(env));
        _blobService = blobService ?? throw new ArgumentNullException(nameof(blobService));
    }

    [HttpGet("/api/v0/apps")]
    public async Task<IActionResult> ListApps()
    {
        var user = this.GetCurrentUserIdentity();
        var apps = await _db.Connection.QueryAsync<Application>(
            @"SELECT a.id, a.name, a.icon_path, a.app_key,
                     a.owner_id = @userId AS has_ownership, a.has_events
              FROM apps a
              LEFT JOIN app_shares s ON s.app_id = a.id
              WHERE (a.owner_id = @userId OR s.email = @userEmail)
              AND a.deleted_at IS NULL
              GROUP BY a.id, a.name, a.icon_path, a.app_key, a.owner_id
              ORDER BY a.name",
            new { userId = user.Id, userEmail = user.Email });

        return Ok(apps);
    }

    [HttpPost("/api/v0/apps")]
    public async Task<IActionResult> Create([FromBody] CreateAppV0RequestBody body)
    {
        var user = this.GetCurrentUserIdentity();
        var app = new Application
        {
            Id = NanoId.New(),
            Name = body.Name,
            AppKey = $"A-{_env.Region}-{NanoId.Numbers(10)}"
        };

        await _db.Connection.ExecuteAsync(
            @"INSERT INTO apps (id, owner_id, name, app_key, has_events)
              VALUES (@appId, @ownerId, @name, @appKey, false)",
            new { appId = app.Id, ownerId = user.Id, name = app.Name, appKey = app.AppKey });

        return Ok(app);
    }

    [HttpGet("/api/v0/apps/{appId}")]
    public async Task<IActionResult> GetById(string appId)
    {
        var app = await GetOwnedOrSharedApp(appId);
        if (app == null)
            return NotFound();

        return Ok(app);
    }

    [HttpPut("/api/v0/apps/{appId}")]
    public async Task<IActionResult> Update(string appId, [FromBody] UpdateAppV0RequestBody body, CancellationToken cancellationToken)
    {
        var app = await GetOwnedApp(appId);
        if (app == null)
            return NotFound();

        if (!string.IsNullOrEmpty(body.Icon))
        {
            var content = Convert.FromBase64String(body.Icon);
            app.IconPath = await _blobService.UploadAsync("icons", content, "image/png", cancellationToken);
        }

        app.Name = body.Name;
        await _db.Connection.ExecuteAsync(
            "UPDATE apps SET name = @name, icon_path = @iconPath WHERE id = @appId",
            new { appId = app.Id, name = app.Name, iconPath = app.IconPath });

        return Ok(app);
    }

    [HttpDelete("/api/v0/apps/{appId}")]
    public async Task<IActionResult> Delete(string appId)
    {
        var app = await GetOwnedApp(appId);
        if (app == null)
            return NotFound();

        await _db.Connection.ExecuteAsync(
            "UPDATE apps SET deleted_at = now() WHERE id = @appId",
            new { appId = app.Id });

        return Ok(new { });
    }

    [HttpGet("/api/v0/apps/{appId}/shares")]
    public async Task<IActionResult> ListShares(string appId)
    {
        var app = await GetOwnedApp(appId);
        if (app == null)
            return NotFound();

        var shares = await _db.Connection.QueryAsync<ApplicationShare>(
            "SELECT email, created_at FROM app_shares WHERE app_id = @appId",
            new { appId });

        return Ok(shares);
    }

    [HttpPut("/api/v0/apps/{appId}/shares/{email}")]
    public async Task<IActionResult> AddShare(string appId, string email)
    {
        var app = await GetOwnedApp(appId);
        if (app == null)
            return NotFound();

        await _db.Connection.ExecuteAsync(
            @"INSERT INTO app_shares (app_id, email)
              VALUES (@appId, @email)
              ON CONFLICT DO NOTHING",
            new { appId, email = email.ToLower() });

        return Ok(new { });
    }

    [HttpDelete("/api/v0/apps/{appId}/shares/{email}")]
    public async Task<IActionResult> RemoveShare(string appId, string email)
    {
        var app = await GetOwnedApp(appId);
        if (app == null)
            return NotFound();

        await _db.Connection.ExecuteAsync(
            "DELETE FROM app_shares WHERE app_id = @appId AND email = @email",
            new { appId, email = email.ToLower() });

        return Ok(new { });
    }

    private async Task<Application?> GetOwnedApp(string appId)
    {
        var user = this.GetCurrentUserIdentity();
        return await _db.Connection.QueryFirstOrDefaultAsync<Application>(
            @"SELECT id, name, icon_path, app_key, true as has_ownership, has_events
              FROM apps
              WHERE id = @appId AND owner_id = @userId AND deleted_at IS NULL",
            new { appId, userId = user.Id });
    }

    private async Task<Application?> GetOwnedOrSharedApp(string appId)
    {
        var user = this.GetCurrentUserIdentity();
        return await _db.Connection.QueryFirstOrDefaultAsync<Application>(
            @"SELECT a.id, a.name, a.icon_path, a.app_key,
                     a.owner_id = @userId as has_ownership, a.has_events
              FROM apps a
              LEFT JOIN app_shares s ON s.app_id = a.id
              WHERE a.id = @appId
              AND (a.owner_id = @userId OR s.email = @userEmail)
              AND a.deleted_at IS NULL
              GROUP BY a.id, a.name, a.icon_path, a.app_key, a.owner_id",
            new { appId, userId = user.Id, userEmail = user.Email });
    }
}
```

If `Application` (in `src/Features/Apps/AppQueries.cs` or similar) lacks
any field referenced above (`IconPath`, `HasOwnership`, `HasEvents`),
match its actual shape instead of the assumed one — it must already
support these since `AppsController` (existing, in the same folder) uses
them identically.

- [ ] **Step 5: Run test to verify it passes**

```bash
cd ~/Dev/aptabase-plus && dotnet test tests/IntegrationTests --filter AppsV0Tests
```

Expected: PASS (2 tests). Re-run the full suite once more:

```bash
dotnet test tests/UnitTests tests/IntegrationTests
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git -C ~/Dev/aptabase-plus add src/Features/Apps/AppsV0Controller.cs tests/IntegrationTests/Clients/AccountClient.cs tests/IntegrationTests/AppsV0Tests.cs
git -C ~/Dev/aptabase-plus commit -m "feat: add /api/v0/apps management endpoints"
```

---

## Task 7: Settings UI for API key management

**Files:**
- Create: `src/webapp/features/settings/api_keys/ApiKeysPage.tsx`
- Create: `src/webapp/features/settings/api_keys/useApiKeys.ts`
- Modify: `src/webapp/router.tsx` (add a route)
- Modify: whichever file renders the existing settings navigation (find it
  first — see Step 1)

**Interfaces:**
- Consumes: `/api/v0/api-keys` GET/POST/DELETE (Task 5)
- Produces: a reachable "/settings/api-keys" (or matching existing
  settings URL convention) page

No automated frontend test exists in this repo (no `*.test.ts(x)` files
present) — this task is verified manually via `npm run dev`, not a test
run.

- [ ] **Step 1: Locate the existing settings page and its routing pattern**

```bash
grep -rn "settings" src/webapp/router.tsx
grep -rln "settings" src/webapp/features
```

Read whatever page component and router entry those turn up (e.g. an
account/profile settings page) to match its exact conventions: how it
fetches data (raw `fetch`, a shared API client, react-query, etc.), how it
lays out a list + a modal/dialog, and where nav links to settings pages
live. Use the same conventions in this task instead of the plain
`fetch`-based sketch below if they differ.

- [ ] **Step 2: Write the data hook**

```typescript
import { useCallback, useEffect, useState } from "react";

export interface ApiKeySummary {
  id: string;
  name: string;
  keyPrefix: string;
  lastUsedAt: string | null;
  expiresAt: string | null;
  createdAt: string;
}

export interface ApiKeyCreated extends ApiKeySummary {
  key: string;
}

export function useApiKeys() {
  const [keys, setKeys] = useState<ApiKeySummary[]>([]);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    setLoading(true);
    const response = await fetch("/api/v0/api-keys");
    const data: ApiKeySummary[] = await response.json();
    setKeys(data);
    setLoading(false);
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const createKey = useCallback(
    async (name: string, expiresAt?: string): Promise<ApiKeyCreated> => {
      const response = await fetch("/api/v0/api-keys", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, expiresAt: expiresAt ?? null }),
      });
      const created: ApiKeyCreated = await response.json();
      await refresh();
      return created;
    },
    [refresh]
  );

  const deleteKey = useCallback(
    async (id: string) => {
      await fetch(`/api/v0/api-keys/${id}`, { method: "DELETE" });
      await refresh();
    },
    [refresh]
  );

  return { keys, loading, createKey, deleteKey };
}
```

- [ ] **Step 3: Write the page component**

```tsx
import { useState } from "react";
import { useApiKeys } from "./useApiKeys";

export function ApiKeysPage() {
  const { keys, loading, createKey, deleteKey } = useApiKeys();
  const [newName, setNewName] = useState("");
  const [justCreatedKey, setJustCreatedKey] = useState<string | null>(null);

  async function handleCreate() {
    if (!newName.trim()) return;
    const created = await createKey(newName.trim());
    setJustCreatedKey(created.key);
    setNewName("");
  }

  return (
    <div>
      <h2>API Keys</h2>
      <p>
        API keys grant full access to your account, equivalent to being
        signed in. Anyone with a key can create, modify, or delete apps and
        shares on your behalf.
      </p>

      {justCreatedKey && (
        <div role="alert">
          <strong>Copy this key now — it will not be shown again:</strong>
          <code>{justCreatedKey}</code>
          <button onClick={() => setJustCreatedKey(null)}>Done</button>
        </div>
      )}

      <div>
        <input
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          placeholder="Key name (e.g. terraform-ci)"
        />
        <button onClick={handleCreate}>Create key</button>
      </div>

      {loading ? (
        <p>Loading…</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Prefix</th>
              <th>Last used</th>
              <th>Expires</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {keys.map((key) => (
              <tr key={key.id}>
                <td>{key.name}</td>
                <td>
                  <code>{key.keyPrefix}…</code>
                </td>
                <td>{key.lastUsedAt ?? "never"}</td>
                <td>{key.expiresAt ?? "never"}</td>
                <td>
                  <button onClick={() => deleteKey(key.id)}>Revoke</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
```

- [ ] **Step 4: Wire the route**

Add a route entry to `src/webapp/router.tsx` following the exact pattern
of the existing settings route found in Step 1 — same parent layout
element, same lazy-loading convention if one is used — pointing at
`ApiKeysPage`. Add a corresponding nav link in whatever component renders
the settings navigation (also found in Step 1).

- [ ] **Step 5: Manually verify**

```bash
cd ~/Dev/aptabase-plus/src
npm install
dotnet watch &   # backend
npm run dev      # frontend, in a second terminal
```

Sign in, navigate to the new settings page, create a key, confirm it's
listed with the correct prefix and the plaintext is shown once, revoke
it, confirm it disappears from the list.

- [ ] **Step 6: Commit**

```bash
git -C ~/Dev/aptabase-plus add src/webapp/features/settings/api_keys src/webapp/router.tsx <the nav file found in Step 1>
git -C ~/Dev/aptabase-plus commit -m "feat: add API key management settings page"
```

---

## Task 8: Release pipeline

**Files:**
- Create: `GitVersion.yml`
- Create: `.github/workflows/release.yml`

**Interfaces:**
- Produces: `ghcr.io/nmehlei/aptabase-plus:vX.Y.Z` and `:latest` Docker images on every push to `main`

- [ ] **Step 1: Add `GitVersion.yml`**

```yaml
mode: Mainline
branches:
  main:
    regex: ^main$
    increment: Patch
    is-mainline: true
```

- [ ] **Step 2: Add the release workflow**

```yaml
name: release

# Every push to main gets a new version and a Docker image: GitVersion
# (Mainline mode, see GitVersion.yml) computes the next SemVer, this job
# tags it, then builds/pushes the image in the same job/checkout.
#
# Deliberately one workflow, not two: a workflow that pushes a tag using
# the default GITHUB_TOKEN does NOT trigger other workflows, so tagging
# and building happen in the same job (see terraform-provider-bugsink's
# release.yml for the same pattern and rationale).

on:
  push:
    branches: [main]
  workflow_dispatch:

permissions:
  contents: write
  packages: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Install GitVersion
        uses: gittools/actions/gitversion/setup@v3
        with:
          versionSpec: "5.x"

      - name: Determine version
        id: gitversion
        uses: gittools/actions/gitversion/execute@v3

      - name: Tag this commit
        id: tag
        env:
          VERSION: v${{ steps.gitversion.outputs.semVer }}
        run: |
          if git rev-parse "$VERSION" >/dev/null 2>&1; then
            echo "Tag $VERSION already exists, skipping."
            echo "skip=true" >> "$GITHUB_OUTPUT"
            exit 0
          fi
          git config user.name "github-actions[bot]"
          git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
          git tag -a "$VERSION" -m "$VERSION"
          git push origin "$VERSION"
          echo "skip=false" >> "$GITHUB_OUTPUT"
          echo "version=$VERSION" >> "$GITHUB_OUTPUT"

      - name: Log in to GHCR
        if: steps.tag.outputs.skip != 'true'
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push image
        if: steps.tag.outputs.skip != 'true'
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: |
            ghcr.io/nmehlei/aptabase-plus:${{ steps.tag.outputs.version }}
            ghcr.io/nmehlei/aptabase-plus:latest
```

- [ ] **Step 3: Verify the existing `ci.yml` still passes**

```bash
cd ~/Dev/aptabase-plus && git log --oneline -1
```

Push the branch and confirm the existing `ci.yml` workflow (unit +
integration tests) passes on GitHub Actions before merging to `main` — do
not merge with a red CI run.

- [ ] **Step 4: Commit**

```bash
git -C ~/Dev/aptabase-plus add GitVersion.yml .github/workflows/release.yml
git -C ~/Dev/aptabase-plus commit -m "feat: add GitVersion + GHCR release workflow"
```

---

## Task 9: Upstream proposal document

**Files:**
- Create: `docs/upstream-proposal.md`

- [ ] **Step 1: Write the proposal**

```markdown
# Upstream proposal: API keys for account-management endpoints

Tracks [aptabase/aptabase#145](https://github.com/aptabase/aptabase/issues/145).

## Problem

Aptabase's account-management endpoints (`/api/_apps` and friends) are
authenticated only via an ASP.NET Core cookie session, set through
GitHub/Google OAuth or a magic-link email flow. There is no way for
non-interactive tooling (CI, Terraform, scripts) to authenticate.

## Proposal

- One key type: user-scoped, inheriting the full permissions of its
  owning user (including minting/revoking further keys for that user).
- Format `aptb_<32 random bytes, base64url>`, stored as a SHA-256 hash
  plus a 12-character display prefix; plaintext shown once at creation.
- New `api_keys` table (`id`, `user_id`, `name`, `key_hash`, `key_prefix`,
  `last_used_at`, `expires_at`, timestamps).
- New ASP.NET Core authentication scheme selected via a policy scheme
  based on the presence of `Authorization: Bearer` — every existing
  `[IsAuthenticated]` endpoint accepts a key with zero changes.
- A new, additive `/api/v0/apps` and `/api/v0/api-keys` surface, kept
  separate from the internal `/api/_apps` routes so the SPA team can keep
  reshaping those freely without breaking automation that depends on a
  stable contract.

## Status

Implemented and running in
[aptabase-plus](https://github.com/nmehlei/aptabase-plus), a downstream
distribution. Posted as a comment on #145 on <DATE> — a scoped-down PR is
available on request.

## Relation to the original #145 proposal

The original issue proposed per-app `base64(AppId:ClientSecret)` basic
auth aimed at read-only analytics automation. This proposal uses
account-scoped bearer tokens instead, aimed at write access for
infrastructure-as-code tooling (Terraform). Both could coexist as
separate, differently-scoped credential types if there's interest.
```

- [ ] **Step 2: Commit**

```bash
git -C ~/Dev/aptabase-plus add docs/upstream-proposal.md
git -C ~/Dev/aptabase-plus commit -m "docs: write upstream proposal for API-key authentication"
```

- [ ] **Step 3: Post to upstream — requires explicit user go-ahead**

Posting a comment to `aptabase/aptabase#145` is a public action under the
user's GitHub identity on a repository they don't own. Do not post
without asking first, even though the content was pre-approved here.
When approved:

```bash
gh issue comment 145 --repo aptabase/aptabase --body-file docs/upstream-proposal.md
```

---

## Post-plan verification

```bash
cd ~/Dev/aptabase-plus
docker compose up -d
dotnet build
dotnet test tests/UnitTests tests/IntegrationTests
```

Expected: build succeeds, full test suite passes. This confirms the fork
is ready for `terraform-provider-aptabase`'s acceptance tests to run
against a tagged image of it (see the provider's own implementation
plan).
