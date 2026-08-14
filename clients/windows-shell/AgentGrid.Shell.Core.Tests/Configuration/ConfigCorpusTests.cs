using System.Text.Json;
using System.Text.Json.Serialization;
using AgentGrid.Shell.Core.Configuration;

namespace AgentGrid.Shell.Core.Tests.Configuration;

// Contract: contract-client-config-parity
// (adr-20260814-shared-go-client-config-module).
//
// This is the C# reader of the shared expected-outcome corpus. The Go reader is
// clients/goclient/clientconfig/corpus_test.go and the TypeScript reader is
// clients/workspace/tests/config-corpus.test.ts. All three read the same file
// and assert the same record, which is what makes "the three loaders agree" a
// mechanical property instead of a claim in a README.
//
// It replaces an assertion that each shared fixture merely threw — three
// loaders that each throw can still reject the same document for different
// reasons and accept it with different results.
public sealed class ConfigCorpusTests : IDisposable
{
    private readonly string _directory =
        Path.Combine(Path.GetTempPath(), $"agent-grid-corpus-{Guid.NewGuid():N}");

    private sealed record CorpusServer(
        [property: JsonPropertyName("index")] int Index,
        [property: JsonPropertyName("display_name")] string DisplayName,
        [property: JsonPropertyName("admission")] string Admission,
        [property: JsonPropertyName("enabled")] bool Enabled,
        [property: JsonPropertyName("default_execution")] bool DefaultExecution,
        [property: JsonPropertyName("canonical_url")] string? CanonicalUrl,
        [property: JsonPropertyName("server_id")] string? ServerId,
        [property: JsonPropertyName("rule")] string? Rule);

    private sealed record CorpusExpectation(
        [property: JsonPropertyName("decision")] string Decision,
        [property: JsonPropertyName("servers")] CorpusServer[]? Servers);

    private sealed record CorpusCase(
        [property: JsonPropertyName("name")] string Name,
        [property: JsonPropertyName("rules")] string[] Rules,
        [property: JsonPropertyName("document")] string Document,
        [property: JsonPropertyName("expected")] CorpusExpectation Expected);

    private sealed record CorpusFile(
        [property: JsonPropertyName("cases")] CorpusCase[] Cases);

    private static CorpusFile ReadCorpus()
    {
        var path = Path.Combine(AppContext.BaseDirectory, "Fixtures", "servers-corpus.json");
        return JsonSerializer.Deserialize<CorpusFile>(File.ReadAllText(path))
            ?? throw new InvalidOperationException($"corpus is empty: {path}");
    }

    // Names rather than cases: a theory parameter xunit can serialise keeps the
    // failing case identifiable from the test name alone.
    public static TheoryData<string> CaseNames()
    {
        var data = new TheoryData<string>();
        foreach (var testCase in ReadCorpus().Cases)
            data.Add(testCase.Name);
        return data;
    }

    [Theory]
    [MemberData(nameof(CaseNames))]
    public void Loader_reaches_the_recorded_outcome(string caseName)
    {
        var testCase = Array.Find(ReadCorpus().Cases, candidate => candidate.Name == caseName)
            ?? throw new InvalidOperationException($"corpus case not found: {caseName}");

        // The desktop loader reads four documents and creates the ones that are
        // absent; seeding servers.json after that leaves only the document under
        // test differing from the defaults.
        DesktopConfigLoader.LoadOrCreate(_directory);
        File.WriteAllText(Path.Combine(_directory, "servers.json"), testCase.Document);

        if (testCase.Expected.Decision == "stop")
        {
            Assert.ThrowsAny<Exception>(() => DesktopConfigLoader.LoadOrCreate(_directory));
            return;
        }

        var config = DesktopConfigLoader.LoadOrCreate(_directory);
        var expected = testCase.Expected.Servers ?? [];
        Assert.Equal(expected.Length, config.Servers.Count);
        for (var position = 0; position < expected.Length; position++)
        {
            var want = expected[position];
            var got = config.Servers[position];
            Assert.Equal(want.Index, got.Index);
            Assert.Equal(want.DisplayName, got.DisplayName);
            Assert.Equal(want.Admission, got.Admission);
            if (want.Admission == AdmissionStates.ConfigurationInvalid)
            {
                Assert.NotNull(got.Violation);
                Assert.Equal(want.Rule, got.Violation!.Rule);
                continue;
            }
            Assert.Equal(want.Enabled, got.Enabled);
            Assert.Equal(want.DefaultExecution, got.DefaultExecution);
            Assert.Equal(want.CanonicalUrl, got.Url!.ToString().TrimEnd('/'));
            Assert.Equal(want.ServerId, got.Id);
        }
    }

    public void Dispose()
    {
        if (Directory.Exists(_directory))
            Directory.Delete(_directory, recursive: true);
    }
}
