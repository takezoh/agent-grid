// Executable SessionTarget discriminator conformance for the generated C# SDK.
// Invoked via clients/sdk/scripts/run-session-target-conformance.sh (and CI).
using System.Text.Json;
using AgentGrid.Client.Models;

static void ExpectValid(string label, string json)
{
    try
    {
        var target = JsonSerializer.Deserialize<SessionTarget>(json)
            ?? throw new Exception("deserialize returned null");
        if (!target.IsValid())
            throw new Exception("expected valid SessionTarget");
        Console.WriteLine($"PASS {label}");
    }
    catch (Exception ex)
    {
        throw new Exception($"{label}: expected success, got {ex.GetType().Name}: {ex.Message}", ex);
    }
}

static void ExpectInvalid(string label, string json)
{
    try
    {
        _ = JsonSerializer.Deserialize<SessionTarget>(json);
        throw new Exception($"{label}: expected rejection, converter accepted payload");
    }
    catch (JsonException)
    {
        Console.WriteLine($"PASS {label}");
    }
}

// valid project
ExpectValid(
    "valid-project",
    """{"source":"project","project":{"server_id":"srv-a","project":"/repo/demo"}}""");

// valid repository
ExpectValid(
    "valid-repository",
    """{"source":"repository","repository":{"repository_id":"42","owner":"acme","name":"widgets"}}""");

// collapsed project+repository
ExpectInvalid(
    "collapsed-project-repository",
    """{"source":"project","project":{"server_id":"srv-a","project":"/repo/demo"},"repository":{"repository_id":"42","owner":"acme","name":"widgets"}}""");

// mixed: source repository with project body
ExpectInvalid(
    "mixed-repository-source-project-body",
    """{"source":"repository","project":{"server_id":"srv-a","project":"/repo/demo"}}""");

// mixed: source project with repository body only
ExpectInvalid(
    "mixed-project-source-repository-body",
    """{"source":"project","repository":{"repository_id":"42","owner":"acme","name":"widgets"}}""");

// missing source
ExpectInvalid("missing-source", """{}""");

// unknown source
ExpectInvalid(
    "unknown-source",
    """{"source":"other","project":{"server_id":"srv-a","project":"/repo/demo"}}""");

// empty required identity fields (project)
ExpectInvalid(
    "empty-project-identity",
    """{"source":"project","project":{"server_id":"","project":"/repo/demo"}}""");
ExpectInvalid(
    "empty-project-path",
    """{"source":"project","project":{"server_id":"srv-a","project":""}}""");

// empty required identity fields (repository)
ExpectInvalid(
    "empty-repository-id",
    """{"source":"repository","repository":{"repository_id":"","owner":"acme","name":"widgets"}}""");
ExpectInvalid(
    "empty-repository-owner",
    """{"source":"repository","repository":{"repository_id":"42","owner":"","name":"widgets"}}""");
ExpectInvalid(
    "empty-repository-name",
    """{"source":"repository","repository":{"repository_id":"42","owner":"acme","name":""}}""");

Console.WriteLine("SessionTarget C# conformance: all cases passed");
