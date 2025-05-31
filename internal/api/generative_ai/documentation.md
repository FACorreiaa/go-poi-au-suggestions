# WanderWiseAI - Generative AI Integration (`generativeAI` Package)

This document outlines how the WanderWiseAI Go backend utilizes the `google/generative-ai-go` (`go-genai`) SDK to power personalized itinerary generation and conversational refinements. Our approach focuses on combining the strengths of Large Language Models (LLMs) like Gemini for planning and natural language processing with our structured PostgreSQL/PostGIS database as the source of truth for Point of Interest (POI) data.

## Core Principles

1.  **LLM for Planning & Language:** Gemini is used for understanding user requests, generating itinerary structures, creating descriptive text, and processing natural language in conversations.
2.  **Database as Source of Truth:** Our PostgreSQL/PostGIS database is the authoritative source for POI details (names, **verified coordinates**, addresses, opening hours, tags, etc.).
3.  **Function Calling for Data Retrieval:** Gemini uses "Function Calling" to request specific data from our backend (which then queries the database). This ensures accuracy and grounds the AI's responses in our verified data.
4.  **Context is Key:** User preferences (from selected profiles), existing itinerary state, and conversation history are crucial context provided to Gemini for relevant and personalized outputs.
5.  **Iterative Seeding:** If POI data (like coordinates or initial descriptions) is missing from our database, the AI can be used to initially source this information, which is then **saved to our database** for future use.

## I. Initial Itinerary Generation

This flow describes generating a new itinerary based on a user's request (e.g., "Berlin, 3km around me, interests: museums, art, chill").

**Steps:**

1.  **Receive User Request:**
    *   The API HandlerImpl receives the city, user's current location (optional), selected `user_preference_profile_id`, available time, and any ad-hoc preference overrides.

2.  **Prepare Context for AI:**
    *   **Fetch User Preference Profile:** Load the specified `user_preference_profile` from the database (includes interests, search radius, preferred pace, time, etc.).
    *   **(Optional but Recommended) Pre-filter Candidate POIs:**
        *   Query PostGIS for POIs in the target city that match basic criteria from the user's profile (e.g., types related to 'museums', 'art') and are within the `search_radius_km` of the user's current location (using `ST_DWithin`).
        *   This provides Gemini with a relevant, manageable list of potential POIs to consider, improving response quality and reducing hallucination.
    *   **Define Available Tools (Functions):** Declare functions that Gemini can call. Essential functions include:
        *   `get_poi_details_from_db(poi_name: string, city_name: string)` or `get_poi_details_by_id(poi_id: string)`: Returns structured data for a POI from our database (coordinates, address, actual opening hours, your verified `ai_summary` or description, relevant tags).
        *   *(Initially, if DB is empty)* `fetch_external_poi_info(poi_name: string, city_name: string)`: A function that, if a POI isn't in our DB or lacks critical info (like coordinates), uses Gemini (or another API like Google Places) in a sub-call to get these details. **Crucially, the output of this function call should be saved to our `points_of_interest` table.**

3.  **Construct Prompt for Gemini (`go-genai`):**
    *   Provide clear instructions, user preferences (from the profile), time constraints, city, and (if pre-filtered) the list of candidate POI names/IDs.
    *   **Request Structured Output (e.g., JSON):** Instruct Gemini to return the itinerary as a structured JSON object. This makes parsing reliable.
        ```json
        // Example desired JSON output from Gemini
        {
          "itinerary_title": "Artistic Afternoon in Berlin",
          "estimated_duration_hours": 3,
          "points_of_interest_sequence": [ // AI decides the POIs and their order
            {"name": "Pergamon Museum", "reasoning": "Matches 'museum' interest..."},
            {"name": "East Side Gallery", "reasoning": "Matches 'art' interest..."}
            // ... more POI names or IDs if AI knows them
          ],
          "overall_summary": "A relaxed tour focusing on..."
        }
        ```
    *   Inform Gemini it can use the declared functions (e.g., `get_poi_details_from_db`) to get specific, accurate data for POIs it chooses.

4.  **Interact with Gemini (Handling Function Calls):**
    *   Send the prompt using `go-genai`.
    *   If Gemini responds with a `FunctionCall` (e.g., requesting details for "Pergamon Museum"):
        *   Your Go backend executes the corresponding local Go function (which queries your PostGIS database for "Pergamon Museum").
        *   Send the data retrieved from your database back to Gemini as a `FunctionResponse`.
    *   Repeat if Gemini makes further function calls.
    *   Eventually, Gemini will return its final response (the structured itinerary JSON and/or narrative text).

5.  **Process and Store AI Response:**
    *   Parse the structured JSON from Gemini.
    *   For each POI *name* in the itinerary sequence from Gemini:
        *   If you haven't already fetched its full details via function calling, do so now by querying your `points_of_interest` table (using the name and city to find the correct record). The AI might suggest a POI for which it didn't explicitly call a function if it felt confident.
        *   If the POI (or its coordinates) was just seeded into your DB from an `fetch_external_poi_info` call, ensure you have its `id`.
    *   Store the generated itinerary structure in your `itineraries` and `itinerary_pois` tables, linking to the `poi_id`s from your database. Store any AI-generated narrative (`ai_description` in `itinerary_pois`).
    *   Log the interaction in `llm_interactions`.

6.  **Return to User:** Send the processed itinerary (map data derived from your DB coordinates, textual descriptions from Gemini/your DB) to the frontend.

## II. Multi-Turn Conversational Refinements (Chat)

This flow handles user follow-up requests within an active itinerary planning session (e.g., "Change to 5km around me and display more info").

**Prerequisites:**
*   An initial itinerary has been generated and its state (current POIs, constraints used) is being managed.
*   A `session_id` tracks the ongoing conversation.

**Steps:**

1.  **Session & State Management (Go Backend):**
    *   **Storage:** Use Redis (recommended for speed) or PostgreSQL (`JSONB`) to store `SessionState` keyed by `session_id`.
    *   `SessionState` struct should include:
        *   `CurrentItinerary`: List of `poi_id`s and key details forming the current plan.
        *   `OriginalUserPreferences`: From the initially selected profile.
        *   `AppliedConstraints`: (e.g., current search radius like "3km", specific interests focused on).
        *   `ConversationHistory`: A list of `User` and `AI` turns.

2.  **Receive User's Follow-up Request:**
    *   E.g., "Okay, now change the radius to 5km and show me some more food options."
    *   The request includes the `session_id`.

3.  **Prepare Context for AI (Contextual Prompt Augmentation):**
    *   **Load SessionState:** Retrieve the user's current session using `session_id`.
    *   **Append User Message:** Add the new user message to `ConversationHistory`.
    *   **Construct Prompt for Gemini:**
        *   Include the `CurrentItinerary` (or a summary).
        *   Include relevant `ConversationHistory` (the last few turns are often sufficient).
        *   Include `AppliedConstraints`.
        *   Clearly state the user's new refinement request.
        *   Example Instruction: "Given the previous conversation, the current itinerary ([...]), and current constraints (radius: 3km), the user now requests: 'Change to 5km around me and display more info [related to food]'. Please update the itinerary based on this new 5km radius and suggest additional food options, using available functions to get POI details."
    *   Ensure the same `Tool` definitions (function declarations) are available to Gemini.

4.  **Interact with Gemini (Handling Function Calls):**
    *   Same as in initial generation: send prompt, handle function calls (e.g., Gemini might call `get_poi_details_from_db` for new POIs within the 5km radius or for existing POIs to get "more info"), receive final response.
    *   Gemini should ideally output the *modified* itinerary structure or the *additional* POIs.

5.  **Process and Update State:**
    *   Parse Gemini's response.
    *   Update the `CurrentItinerary` and `AppliedConstraints` (e.g., radius is now "5km") in the `SessionState`.
    *   Save the updated `SessionState` back to Redis/Postgres.
    *   Log the interaction in `llm_interactions`.
    *   Update the `itineraries` and `itinerary_pois` tables in your main database if the canonical itinerary has changed significantly (or manage this as a "draft" itinerary within the session until the user confirms).

6.  **Return to User:** Send the updated itinerary/information.

## Key `go-genai` Considerations:

*   **Chat vs. Non-Chat Models:** For multi-turn, use Gemini's chat models (`StartChat()` method in `go-genai`), as they are designed to handle conversation history.
*   **Prompt Clarity:** Be extremely clear in your prompts, especially when requesting structured output (like JSON) or guiding function usage.
*   **Function Descriptions:** Make the `Description` field in your `genai.FunctionDeclaration` very informative so Gemini knows when and how to use your tools.
*   **Error Handling:** Implement robust error handling for API calls and function executions.
*   **Token Limits:** Be mindful of context window limits. For long conversations, you might need strategies to summarize or select relevant parts of the history.

By following these steps, your `generativeAI` package can effectively use `go-genai` to create a powerful and personalized city discovery experience.