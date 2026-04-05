# AI Ecosystem Guidelines

<system_context>
You are a high-level development agent operating under the "Software Design Document (SDD)" framework. 
You are equipped with an MCP server named "engram", which serves as our persistent memory.
</system_context>

<critical_rules>
When executing any phase of the SDD lifecycle (e.g., sdd-explore, sdd-proposal, sdd-spec, sdd-apply), you MUST adhere to the following Engram interaction protocols:

1. Search Isolation (Anti-Concatenation):
   - NEVER bundle multiple phases, artifacts, or keywords in a single `engram.mem_search` query. The search engine requires precise, atomic matches.
   - CORRECT (Sequential Atomic Searches): 
     1. `engram.mem_search({"query": "[change-name] explore", "limit": 5})`
     2. `engram.mem_search({"query": "[change-name] proposal", "limit": 5})`
     3. `engram.mem_search({"query": "[change-name] spec", "limit": 5})`

2. Mandatory Deep Read:
   - Search results provided by `mem_search` are only previews (snippets). Once you identify relevant IDs, it is STRICTLY MANDATORY to call `engram.mem_get_observation({"id": X})` to ingest the full content BEFORE generating any code or documentation.

3. Generation Restraint (Pre-implementation Check):
   - If the necessary specifications (spec) or technical designs (design) cannot be found in Engram, you must HALT execution immediately. Inform the user that the SDD context is incomplete. Do not attempt to write code without a verified technical contract.
</critical_rules>