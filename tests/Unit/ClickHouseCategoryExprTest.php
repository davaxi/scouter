<?php

use App\Analysis\CategoryExpr;

/**
 * Unit tests for the live ClickHouse categorization expression builder.
 *
 * CategoryExpr::build() turns parsed YAML rules into a ClickHouse CASE WHEN
 * (RE2 match(), inlined path replace) — the CH counterpart of the PG
 * CategorizationService::buildCaseWhenSql(). These tests pin the dialect
 * translation and the first-match-wins ordering without touching a database.
 */

function makeCategoryExpr(): CategoryExpr
{
    // The constructor stores the PDO only for forCrawl(); build() never uses it,
    // so a deliberately unusable handle is fine for these pure-logic tests.
    $ref = new ReflectionClass(CategoryExpr::class);
    return $ref->newInstanceWithoutConstructor();
}

it('returns empty-string literal when there are no rules', function () {
    expect(makeCategoryExpr()->build([]))->toBe("''");
});

it('builds a case-insensitive RE2 match on url + path for a simple rule', function () {
    $sql = makeCategoryExpr()->build([
        ['name' => 'product', 'domain' => 'shop.com', 'includes' => ['^/p/'], 'excludes' => []],
    ]);

    expect($sql)->toStartWith('CASE WHEN ');
    expect($sql)->toEndWith(" ELSE '' END");
    // domain match, case-insensitive. preg_quote escapes the dot (shop\.com), then
    // lit() doubles every backslash so the CH string literal delivers the exact
    // bytes to RE2 (a lone "\x" could otherwise be eaten as a CH string escape).
    expect($sql)->toContain("match(url, '(?i)shop\\\\.com')");
    // include matched against the path (host stripped via replaceRegexpOne):
    expect($sql)->toContain("replaceRegexpOne(url, '^https?://[^/]+', '')");
    expect($sql)->toContain("'(?i)^/p/'");
    expect($sql)->toContain("THEN 'product'");
});

it('adds a negated match for excludes', function () {
    $sql = makeCategoryExpr()->build([
        ['name' => 'blog', 'domain' => 'x.com', 'includes' => ['^/blog'], 'excludes' => ['/tag/', '/author/']],
    ]);

    expect($sql)->toContain("AND NOT match(replaceRegexpOne(url, '^https?://[^/]+', ''), '(?i)/tag/|/author/')");
});

it('preserves rule order (first match wins) and ORs multiple includes', function () {
    $sql = makeCategoryExpr()->build([
        ['name' => 'a', 'domain' => 'x.com', 'includes' => ['^/a', '^/aa'], 'excludes' => []],
        ['name' => 'b', 'domain' => 'x.com', 'includes' => ['^/b'], 'excludes' => []],
    ]);

    expect($sql)->toContain("'(?i)^/a|^/aa'");
    expect(strpos($sql, "THEN 'a'"))->toBeLessThan(strpos($sql, "THEN 'b'"));
});

it('escapes single quotes in category names and patterns', function () {
    $sql = makeCategoryExpr()->build([
        ['name' => "O'Reilly", 'domain' => 'x.com', 'includes' => ["/o'r"], 'excludes' => []],
    ]);

    // doubled quotes inside the SQL string literals
    expect($sql)->toContain("THEN 'O''Reilly'");
    expect($sql)->toContain("/o''r");
});

it('has no hidden condition when no segment is hidden', function () {
    expect(makeCategoryExpr()->buildHiddenCond([
        ['name' => 'blog', 'domain' => 'x.com', 'includes' => ['^/blog'], 'excludes' => []],
    ]))->toBeNull();
});

it('matches the pages whose first matching segment is hidden', function () {
    $cond = makeCategoryExpr()->buildHiddenCond([
        ['name' => 'blog', 'domain' => 'x.com', 'includes' => ['^/blog'], 'excludes' => []],
        ['name' => 'cf', 'domain' => 'x.com', 'includes' => ['^/cdn-cgi/'], 'excludes' => [], 'hidden' => true],
        ['name' => 'rest', 'domain' => 'x.com', 'includes' => ['.*'], 'excludes' => []],
    ]);

    // First-match ids (as for category): a /blog page stays id 1 even if it
    // also matched a later hidden rule.
    expect($cond)->toStartWith('ifNull(CASE WHEN ');
    expect($cond)->toEndWith(', 0) IN (2)');
    expect($cond)->toContain("'(?i)^/cdn-cgi/'");
    // Rules after the last hidden one can't change the outcome → trimmed.
    expect($cond)->not->toContain("'(?i).*'");
});

it('detects a segment flagged hidden in a parsed YAML config', function () {
    $has = fn(string $yaml) => \App\Analysis\CategorizationService::hasHiddenSegment(\Spyc::YAMLLoadString($yaml));

    expect($has("blog:\n  include:\n    - ^/blog\n"))->toBeFalse();
    expect($has("cf:\n  hidden: true\n  include:\n    - ^/cdn-cgi/\n"))->toBeTrue();
    expect($has("cf:\n  hidden: false\n  include:\n    - ^/cdn-cgi/\n"))->toBeFalse();
    expect(\App\Analysis\CategorizationService::hasHiddenSegment(null))->toBeFalse();
});
