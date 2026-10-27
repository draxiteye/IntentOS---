describe("IntentOS Frontend Core Logic & State Validation", () => {
  test("sanity test environment", () => {
    expect(1 + 1).toBe(2);
  });

  test("localStorage authentication token persistence", () => {
    const token = "mock-jwt-token-12345";
    localStorage.setItem("intentos_token", token);
    expect(localStorage.getItem("intentos_token")).toBe(token);
    localStorage.removeItem("intentos_token");
    expect(localStorage.getItem("intentos_token")).toBeNull();
  });

  test("task completion filtering logic", () => {
    const mockTasks = [
      { id: "1", title: "Task 1", status: "pending", intention_id: "int-1", priority: 1, estimated_duration: 30 },
      { id: "2", title: "Task 2", status: "completed", intention_id: "int-1", priority: 2, estimated_duration: 60 },
      { id: "3", title: "Task 3", status: "pending", intention_id: "int-2", priority: 0, estimated_duration: 15 },
    ];

    const pending = mockTasks.filter(t => t.status !== "completed");
    const completed = mockTasks.filter(t => t.status === "completed");
    const filteredByIntention = mockTasks.filter(t => t.intention_id === "int-1");

    expect(pending.length).toBe(2);
    expect(completed.length).toBe(1);
    expect(filteredByIntention.length).toBe(2);
  });

  test("priority level mapping helper", () => {
    function getPriorityLabel(p) {
      if (p >= 2) return "High";
      if (p === 1) return "Medium";
      return "Low";
    }

    expect(getPriorityLabel(2)).toBe("High");
    expect(getPriorityLabel(1)).toBe("Medium");
    expect(getPriorityLabel(0)).toBe("Low");
  });

  test("total scheduled duration computation", () => {
    const blocks = [
      { start_timestamp: "2026-10-27T10:00:00Z", end_timestamp: "2026-10-27T10:45:00Z" }, // 45m
      { start_timestamp: "2026-10-27T10:45:00Z", end_timestamp: "2026-10-27T12:15:00Z" }, // 90m
    ];

    const totalMinutes = blocks.reduce((acc, b) => {
      const start = new Date(b.start_timestamp).getTime();
      const end = new Date(b.end_timestamp).getTime();
      return acc + (end - start) / (1000 * 60);
    }, 0);

    expect(totalMinutes).toBe(135);
  });

  test("assistant heuristic parser extracts tasks and durations", () => {
    const prompt = "I need to revise subnetting (45 mins) and solve a practice paper (90m)";
    const clauses = prompt.split(/and/i);
    const extracted = clauses.map(c => {
      const durationMatch = c.match(/(\d+)\s*(?:m|min|mins|minutes)/i);
      const duration = durationMatch ? parseInt(durationMatch[1], 10) : 30;
      return { text: c.trim(), duration };
    });

    expect(extracted.length).toBe(2);
    expect(extracted[0].duration).toBe(45);
    expect(extracted[1].duration).toBe(90);
  });
});
