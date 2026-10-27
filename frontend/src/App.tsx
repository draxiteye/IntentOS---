import { useState, useEffect, type FormEvent } from "react";

const API = (import.meta.env.VITE_API_URL ? import.meta.env.VITE_API_URL.replace(/\/$/, "") : "") + "/api";

interface User {
  id: string;
  email: string;
}

interface Intention {
  id: string;
  user_id: string;
  title: string;
  description?: string;
  horizon: string; // Mastery, Career, Vitality, Ventures
  created_at: string;
  updated_at: string;
}

interface Task {
  id: string;
  intention_id: string;
  intention_title?: string;
  title: string;
  description?: string;
  deadline?: string;
  estimated_duration: number; // minutes
  actual_duration: number;    // minutes
  priority: number;           // 0=Low, 1=Medium, 2=High
  energy_level: string;       // high, medium, low
  status: string;             // pending, in_progress, completed
  created_at: string;
  updated_at: string;
}

interface ScheduleBlock {
  id: string;
  user_id: string;
  task_id: string;
  task_title?: string;
  energy_level?: string;
  start_timestamp: string;
  end_timestamp: string;
}

interface PlanRevision {
  id: string;
  user_id: string;
  created_at: string;
  description: string;
}

type Tab = "overview" | "intentions" | "tasks" | "planner" | "assistant";
type Notice = { kind: "success" | "error"; text: string } | null;

export default function App() {
  const [token, setToken] = useState<string>(() => localStorage.getItem("intentos_token") || "");
  const [user, setUser] = useState<User | null>(() => {
    const saved = localStorage.getItem("intentos_user");
    return saved ? JSON.parse(saved) : null;
  });

  // Auth form state
  const [authMode, setAuthMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  // Navigation
  const [activeTab, setActiveTab] = useState<Tab>("overview");

  // Core Data
  const [intentions, setIntentions] = useState<Intention[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [scheduleBlocks, setScheduleBlocks] = useState<ScheduleBlock[]>([]);
  const [planHistory, setPlanHistory] = useState<PlanRevision[]>([]);
  const [unscheduledWarnings, setUnscheduledWarnings] = useState<string[]>([]);

  // Daily capacity budget in minutes (default 240 mins = 4 hours)
  const [dailyCapacityMinutes, setDailyCapacityMinutes] = useState(240);

  // Modals & States
  const [showIntentionModal, setShowIntentionModal] = useState(false);
  const [newIntentionTitle, setNewIntentionTitle] = useState("");
  const [newIntentionDesc, setNewIntentionDesc] = useState("");
  const [newIntentionHorizon, setNewIntentionHorizon] = useState("Mastery");

  const [showTaskModal, setShowTaskModal] = useState(false);
  const [newTaskIntentionId, setNewTaskIntentionId] = useState("");
  const [newTaskTitle, setNewTaskTitle] = useState("");
  const [newTaskDesc, setNewTaskDesc] = useState("");
  const [newTaskDuration, setNewTaskDuration] = useState(30);
  const [newTaskPriority, setNewTaskPriority] = useState(1);
  const [newTaskEnergy, setNewTaskEnergy] = useState("medium");
  const [newTaskDeadline, setNewTaskDeadline] = useState("");

  // Off-Track Reconciliation Modal
  const [showReconcileModal, setShowReconcileModal] = useState(false);
  const [delayMinutes, setDelayMinutes] = useState(30);
  const [delayReason, setDelayReason] = useState("Task took longer than anticipated");

  // Zen Ambient Focus Mode State
  const [zenFocusTask, setZenFocusTask] = useState<Task | null>(null);
  const [zenTimerSeconds, setZenTimerSeconds] = useState(25 * 60);
  const [zenTimerActive, setZenTimerActive] = useState(false);
  const [parkedThought, setParkedThought] = useState("");

  // Unstick Me Modal
  const [unstickTask, setUnstickTask] = useState<Task | null>(null);
  const [unstickSteps, setUnstickSteps] = useState<string[]>([]);

  // Filter states
  const [selectedIntentionFilter, setSelectedIntentionFilter] = useState<string>("all");
  const [selectedHorizonFilter, setSelectedHorizonFilter] = useState<string>("all");
  const [taskStatusFilter, setTaskStatusFilter] = useState<string>("all");

  // Assistant prompt state
  const [assistantPrompt, setAssistantPrompt] = useState(
    "I have a Computer Networks exam on Friday. I need to finish routing protocols (60 mins, high energy), revise subnetting (45 mins, medium energy), and solve a practice paper (90 mins, high energy)."
  );

  // UI state
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<Notice>(null);

  // Auto-dismiss notices
  useEffect(() => {
    if (!notice) return;
    const timer = setTimeout(() => setNotice(null), 5000);
    return () => clearTimeout(timer);
  }, [notice]);

  // Zen countdown timer effect
  useEffect(() => {
    if (!zenTimerActive || !zenFocusTask) return;
    const interval = setInterval(() => {
      setZenTimerSeconds(prev => {
        if (prev <= 1) {
          clearInterval(interval);
          setZenTimerActive(false);
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(interval);
  }, [zenTimerActive, zenFocusTask]);

  // Unified API caller
  async function apiRequest<T = any>(path: string, method = "GET", body?: unknown): Promise<T> {
    const res = await fetch(`${API}${path}`, {
      method,
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
    });

    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      if (res.status === 401 && token) {
        handleLogout();
        throw new Error("Session expired. Please sign in again.");
      }
      throw new Error(data.error || data.message || `Request failed (${res.status})`);
    }
    return data;
  }

  // Load user data on authentication
  useEffect(() => {
    if (token) {
      loadAllData();
    }
  }, [token]);

  async function loadAllData() {
    setBusy(true);
    try {
      const [fetchedIntentions, fetchedTasks, fetchedBlocks, fetchedHistory] = await Promise.all([
        apiRequest<Intention[]>("/intentions").catch(() => []),
        apiRequest<Task[]>("/tasks").catch(() => []),
        apiRequest<ScheduleBlock[]>("/plan/blocks").catch(() => []),
        apiRequest<PlanRevision[]>("/plan/history").catch(() => []),
      ]);

      setIntentions(fetchedIntentions || []);
      setTasks(fetchedTasks || []);
      setScheduleBlocks(fetchedBlocks || []);
      setPlanHistory(fetchedHistory || []);
    } catch (err: any) {
      setNotice({ kind: "error", text: err.message || "Failed to load workspace data." });
    } finally {
      setBusy(false);
    }
  }

  // Auth handler
  async function handleAuth(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setNotice(null);

    try {
      const endpoint = authMode === "login" ? "/auth/login" : "/auth/register";
      const data = await apiRequest(endpoint, "POST", { email, password });

      const receivedToken = data.token;
      if (!receivedToken) {
        throw new Error("Authentication succeeded but token was missing.");
      }

      localStorage.setItem("intentos_token", receivedToken);
      setToken(receivedToken);

      const userInfo = data.user || { id: "", email };
      localStorage.setItem("intentos_user", JSON.stringify(userInfo));
      setUser(userInfo);

      setNotice({ kind: "success", text: authMode === "login" ? "Welcome back!" : "Account created and logged in." });
      setEmail("");
      setPassword("");
    } catch (err: any) {
      setNotice({ kind: "error", text: err.message || "Authentication failed." });
    } finally {
      setBusy(false);
    }
  }

  function handleLogout() {
    localStorage.removeItem("intentos_token");
    localStorage.removeItem("intentos_user");
    setToken("");
    setUser(null);
    setIntentions([]);
    setTasks([]);
    setScheduleBlocks([]);
    setPlanHistory([]);
    setNotice({ kind: "success", text: "Signed out successfully." });
  }

  // Create Intention
  async function handleCreateIntention(e: FormEvent) {
    e.preventDefault();
    if (!newIntentionTitle.trim()) return;
    setBusy(true);

    try {
      const created = await apiRequest<Intention>("/intentions", "POST", {
        title: newIntentionTitle.trim(),
        description: newIntentionDesc.trim() || undefined,
        horizon: newIntentionHorizon,
      });

      setIntentions(prev => [created, ...prev]);
      setNewIntentionTitle("");
      setNewIntentionDesc("");
      setShowIntentionModal(false);
      setNotice({ kind: "success", text: `Intention "${created.title}" anchored under ${created.horizon}.` });
    } catch (err: any) {
      setNotice({ kind: "error", text: err.message });
    } finally {
      setBusy(false);
    }
  }

  // Delete Intention
  async function handleDeleteIntention(id: string, title: string) {
    if (!confirm(`Delete intention "${title}" and all its linked tasks?`)) return;
    setBusy(true);

    try {
      await apiRequest(`/intentions/${id}`, "DELETE");
      setIntentions(prev => prev.filter(i => i.id !== id));
      setTasks(prev => prev.filter(t => t.intention_id !== id));
      setNotice({ kind: "success", text: `Intention removed.` });
    } catch (err: any) {
      setNotice({ kind: "error", text: err.message });
    } finally {
      setBusy(false);
    }
  }

  // Create Task
  async function handleCreateTask(e: FormEvent) {
    e.preventDefault();
    if (!newTaskTitle.trim() || !newTaskIntentionId) {
      setNotice({ kind: "error", text: "Title and associated intention are required." });
      return;
    }
    setBusy(true);

    try {
      const payload: any = {
        intention_id: newTaskIntentionId,
        title: newTaskTitle.trim(),
        description: newTaskDesc.trim() || undefined,
        estimated_duration: Number(newTaskDuration) || 30,
        priority: Number(newTaskPriority),
        energy_level: newTaskEnergy,
      };

      if (newTaskDeadline) {
        payload.deadline = new Date(newTaskDeadline).toISOString();
      }

      const created = await apiRequest<Task>("/tasks", "POST", payload);
      setTasks(prev => [created, ...prev]);
      setNewTaskTitle("");
      setNewTaskDesc("");
      setNewTaskDeadline("");
      setShowTaskModal(false);
      setNotice({ kind: "success", text: `Task "${created.title}" added to queue.` });
    } catch (err: any) {
      setNotice({ kind: "error", text: err.message });
    } finally {
      setBusy(false);
    }
  }

  // Toggle Task Completion + Actual Duration Logger
  async function handleToggleTaskStatus(task: Task, loggedActual?: number) {
    const nextStatus = task.status === "completed" ? "pending" : "completed";
    try {
      const payload: any = { status: nextStatus };
      if (nextStatus === "completed") {
        payload.actual_duration = loggedActual ?? task.estimated_duration;
      }

      await apiRequest(`/tasks/${task.id}`, "PUT", payload);
      setTasks(prev =>
        prev.map(t =>
          t.id === task.id
            ? { ...t, status: nextStatus, actual_duration: payload.actual_duration ?? t.actual_duration }
            : t
        )
      );

      setNotice({
        kind: "success",
        text:
          nextStatus === "completed"
            ? `Task completed! Logged ${payload.actual_duration} mins.`
            : `Task marked pending.`,
      });
    } catch (err: any) {
      setNotice({ kind: "error", text: err.message });
    }
  }

  // Delete Task
  async function handleDeleteTask(id: string) {
    try {
      await apiRequest(`/tasks/${id}`, "DELETE");
      setTasks(prev => prev.filter(t => t.id !== id));
      setNotice({ kind: "success", text: "Task deleted." });
    } catch (err: any) {
      setNotice({ kind: "error", text: err.message });
    }
  }

  // Generate / Replan
  async function handleGeneratePlan(isReplan = false) {
    setBusy(true);
    setNotice(null);

    try {
      const endpoint = isReplan ? "/plan/replan" : "/plan/generate";
      const data = await apiRequest<{ blocks: ScheduleBlock[]; unscheduled_tasks?: string[] }>(endpoint, "POST");

      setScheduleBlocks(data.blocks || []);
      setUnscheduledWarnings(data.unscheduled_tasks || []);

      const history = await apiRequest<PlanRevision[]>("/plan/history").catch(() => []);
      setPlanHistory(history || []);

      setNotice({
        kind: "success",
        text: `${data.blocks?.length || 0} task blocks scheduled with deterministic topological ordering!`,
      });
    } catch (err: any) {
      setNotice({ kind: "error", text: err.message || "Failed to schedule plan." });
    } finally {
      setBusy(false);
    }
  }

  // Reality Reconciliation (Dynamic Off-Track Adaptive Recovery)
  async function handleReconcileReality(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setNotice(null);

    try {
      const data = await apiRequest<{ blocks: ScheduleBlock[]; explanation?: string; unscheduled_tasks?: string[] }>(
        "/plan/reconcile",
        "POST",
        {
          delay_minutes: Number(delayMinutes),
          reason: delayReason,
        }
      );

      setScheduleBlocks(data.blocks || []);
      setUnscheduledWarnings(data.unscheduled_tasks || []);
      setShowReconcileModal(false);

      const history = await apiRequest<PlanRevision[]>("/plan/history").catch(() => []);
      setPlanHistory(history || []);

      setNotice({
        kind: "success",
        text: data.explanation || "Reality reconciled! Schedule adjusted without guilt.",
      });
    } catch (err: any) {
      setNotice({ kind: "error", text: err.message || "Reconciliation failed." });
    } finally {
      setBusy(false);
    }
  }

  // Zen Mode: Launch Task
  function launchZenFocus(task: Task) {
    setZenFocusTask(task);
    setZenTimerSeconds((task.estimated_duration || 25) * 60);
    setZenTimerActive(true);
  }

  // Zen Mode: Distraction Capture
  async function handleParkThought(e: FormEvent) {
    e.preventDefault();
    if (!parkedThought.trim() || !intentions[0]) return;

    try {
      await apiRequest("/tasks", "POST", {
        intention_id: intentions[0].id,
        title: parkedThought.trim(),
        estimated_duration: 15,
        priority: 0,
        energy_level: "low",
      });
      setParkedThought("");
      loadAllData();
      setNotice({ kind: "success", text: "Thought parked in task inbox. Stay in focus!" });
    } catch (err: any) {
      setNotice({ kind: "error", text: err.message });
    }
  }

  // Agentic "Unstick Me" Micro-Stepper
  function handleUnstickMe(task: Task) {
    setUnstickTask(task);
    // Decompose intimidating goal into 3 5-minute micro-activation steps
    const steps = [
      `Step 1 (5 mins): Open the project or material for "${task.title}". Don't write yet—just scan the outline.`,
      `Step 2 (10 mins): Write down 3 quick, messy bullet points answering: "What is the very first question to solve?"`,
      `Step 3 (15 mins): Run a silent timer and work on only the first bullet point. Zero pressure to finish.`,
    ];
    setUnstickSteps(steps);
  }

  // Assistant Natural Language Parser
  async function handleAssistantSubmit(e: FormEvent) {
    e.preventDefault();
    if (!assistantPrompt.trim()) return;
    setBusy(true);

    try {
      const prompt = assistantPrompt.trim();
      const firstSentence = prompt.split(/[.?!]/)[0] || "Exam Preparation";
      const intentionTitle = firstSentence.length > 50 ? firstSentence.slice(0, 50) + "..." : firstSentence;

      const createdIntention = await apiRequest<Intention>("/intentions", "POST", {
        title: intentionTitle,
        description: prompt,
        horizon: prompt.toLowerCase().includes("exam") || prompt.toLowerCase().includes("learn") ? "Mastery" : "Career",
      });

      const clauses = prompt.split(/[,;\n]|and need to|i need to|need to|and revise|and solve/i);
      const extractedTasks: { title: string; duration: number; energy: string }[] = [];

      for (const clause of clauses) {
        const text = clause.trim();
        if (text.length > 8 && !text.toLowerCase().startsWith("i have a")) {
          let duration = 30;
          const durationMatch = text.match(/(\d+)\s*(?:m|min|mins|minutes|hour|hr)/i);
          if (durationMatch) {
            duration = parseInt(durationMatch[1], 10);
            if (clause.toLowerCase().includes("hour") || clause.toLowerCase().includes("hr")) {
              duration *= 60;
            }
          }

          let energy = "medium";
          if (clause.toLowerCase().includes("high") || clause.toLowerCase().includes("exam") || clause.toLowerCase().includes("solve")) {
            energy = "high";
          } else if (clause.toLowerCase().includes("low") || clause.toLowerCase().includes("clean")) {
            energy = "low";
          }

          const cleanTitle = text
            .replace(/\([^)]*\)/g, "")
            .replace(/\b\d+\s*(?:m|min|mins|minutes|hour|hr)\b/gi, "")
            .replace(/high energy|medium energy|low energy/gi, "")
            .trim();

          if (cleanTitle) {
            extractedTasks.push({ title: cleanTitle, duration, energy });
          }
        }
      }

      if (extractedTasks.length === 0) {
        extractedTasks.push({ title: "Review core objectives", duration: 45, energy: "high" });
      }

      for (const t of extractedTasks) {
        await apiRequest("/tasks", "POST", {
          intention_id: createdIntention.id,
          title: t.title,
          estimated_duration: t.duration,
          priority: t.energy === "high" ? 2 : 1,
          energy_level: t.energy,
        });
      }

      await loadAllData();
      setActiveTab("planner");
      setNotice({
        kind: "success",
        text: `Intention "${createdIntention.title}" parsed with ${extractedTasks.length} tasks! Ready to schedule.`,
      });
    } catch (err: any) {
      setNotice({ kind: "error", text: err.message || "Failed to parse intention." });
    } finally {
      setBusy(false);
    }
  }

  // Computed metrics
  const totalTasks = tasks.length;
  const completedTasks = tasks.filter(t => t.status === "completed").length;
  const pendingTasks = tasks.filter(t => t.status !== "completed");
  const nextUpTask = pendingTasks[0] || null;

  // Total scheduled duration vs capacity (YNAB for Time)
  const scheduledMinutes = scheduleBlocks.reduce((acc, b) => {
    const start = new Date(b.start_timestamp).getTime();
    const end = new Date(b.end_timestamp).getTime();
    return acc + Math.round((end - start) / (1000 * 60));
  }, 0);

  const capacityPercent = Math.min(Math.round((scheduledMinutes / dailyCapacityMinutes) * 100), 100);
  const isOverbooked = scheduledMinutes > dailyCapacityMinutes;

  // Filtered tasks
  const filteredTasks = tasks.filter(t => {
    const matchesIntention = selectedIntentionFilter === "all" || t.intention_id === selectedIntentionFilter;
    const matchesStatus =
      taskStatusFilter === "all" ||
      (taskStatusFilter === "completed" ? t.status === "completed" : t.status !== "completed");
    return matchesIntention && matchesStatus;
  });

  // Helpers
  function formatTime(iso: string) {
    if (!iso) return "";
    try {
      const d = new Date(iso);
      return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    } catch {
      return iso;
    }
  }

  function formatDate(iso: string) {
    if (!iso) return "";
    try {
      const d = new Date(iso);
      return d.toLocaleDateString([], { month: "short", day: "numeric" });
    } catch {
      return iso;
    }
  }

  function formatTimerDigits(totalSeconds: number) {
    const m = Math.floor(totalSeconds / 60);
    const s = totalSeconds % 60;
    return `${m < 10 ? "0" : ""}${m}:${s < 10 ? "0" : ""}${s}`;
  }

  function getPriorityLabel(p: number) {
    if (p >= 2) return { text: "High", cls: "badge-priority-2" };
    if (p === 1) return { text: "Medium", cls: "badge-priority-1" };
    return { text: "Low", cls: "badge-priority-0" };
  }

  function getEnergyBadge(energy: string) {
    if (energy === "high") return { text: "🧠 Deep Work", cls: "badge-energy-high" };
    if (energy === "low") return { text: "☕ Low Energy", cls: "badge-energy-low" };
    return { text: "⚡ Medium", cls: "badge-energy-medium" };
  }

  // ---------------- Render: Auth Screen ----------------
  if (!token) {
    return (
      <div className="auth-container">
        <div className="auth-box">
          <div className="auth-header">
            <div className="brand" style={{ justifyContent: "center", border: "none", padding: "0 0 16px 0", margin: 0 }}>
              <div className="brand-icon">I</div>
              <span className="brand-title">IntentOS</span>
            </div>
            <h1 className="auth-title">{authMode === "login" ? "Welcome back" : "Operating System for Human Intent"}</h1>
            <p className="auth-subtitle">
              {authMode === "login"
                ? "Sign in to synchronize your intentions with reality-grounded scheduling."
                : "Transform messy goals into deterministic daily execution with zero-guilt adaptation."}
            </p>
          </div>

          {notice && <div className={`toast-notice ${notice.kind}`}>{notice.text}</div>}

          <form onSubmit={handleAuth}>
            <div className="form-group">
              <label className="form-label">Email address</label>
              <input
                className="form-input"
                type="email"
                required
                value={email}
                onChange={e => setEmail(e.target.value)}
                placeholder="you@domain.com"
              />
            </div>

            <div className="form-group">
              <label className="form-label">Password</label>
              <input
                className="form-input"
                type="password"
                required
                minLength={6}
                value={password}
                onChange={e => setPassword(e.target.value)}
                placeholder="••••••••"
              />
            </div>

            <button type="submit" className="btn btn-primary" style={{ width: "100%", marginTop: "12px" }} disabled={busy}>
              {busy ? "Authenticating..." : authMode === "login" ? "Sign In →" : "Create Account →"}
            </button>
          </form>

          <div className="auth-footer">
            {authMode === "login" ? "Don't have an account? " : "Already registered? "}
            <button
              onClick={() => {
                setAuthMode(authMode === "login" ? "register" : "login");
                setNotice(null);
              }}
            >
              {authMode === "login" ? "Create one now" : "Sign in here"}
            </button>
          </div>
        </div>
      </div>
    );
  }

  // ---------------- Render: Authenticated Workspace ----------------
  return (
    <div className="app-shell">
      {/* Zen Ambient Focus Mode Overlay */}
      {zenFocusTask && (
        <div className="zen-overlay">
          <button className="zen-close-btn" onClick={() => setZenFocusTask(null)}>
            ✕ Exit Focus Mode
          </button>
          <div className="zen-content">
            <span className="zen-intention-pill">◎ {zenFocusTask.intention_title || "Deep Work"}</span>
            <h2 className="zen-task-title">{zenFocusTask.title}</h2>
            {zenFocusTask.description && <p style={{ color: "var(--text-muted)" }}>{zenFocusTask.description}</p>}

            <div className="zen-timer-ring">
              <div className="zen-timer-digits">{formatTimerDigits(zenTimerSeconds)}</div>
            </div>

            <div style={{ display: "flex", gap: "12px", justifyContent: "center" }}>
              <button
                className="btn btn-secondary"
                onClick={() => setZenTimerActive(!zenTimerActive)}
              >
                {zenTimerActive ? "⏸ Pause" : "▶ Resume"}
              </button>
              <button
                className="btn btn-primary"
                onClick={() => {
                  handleToggleTaskStatus(zenFocusTask);
                  setZenFocusTask(null);
                }}
              >
                ✓ Complete & Next
              </button>
              <button
                className="btn btn-secondary"
                onClick={() => setZenTimerSeconds(prev => prev + 15 * 60)}
              >
                +15 Mins
              </button>
            </div>

            {/* Thought Parking Lot */}
            <div className="zen-parking-lot">
              <div style={{ fontSize: "11px", fontWeight: 700, color: "var(--text-dim)", textTransform: "uppercase" }}>
                Thought Parking Lot (Distraction Capture)
              </div>
              <form onSubmit={handleParkThought}>
                <input
                  className="zen-parking-input"
                  placeholder="Random thought popped into your head? Type it here and press Enter to save..."
                  value={parkedThought}
                  onChange={e => setParkedThought(e.target.value)}
                />
              </form>
            </div>
          </div>
        </div>
      )}

      {/* Sidebar */}
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-icon">I</div>
          <span className="brand-title">IntentOS</span>
          <span className="brand-badge">CORE</span>
        </div>

        <div className="nav-section-title">Cognitive Space</div>
        <nav className="nav-list">
          <button
            className={`nav-button ${activeTab === "overview" ? "active" : ""}`}
            onClick={() => setActiveTab("overview")}
          >
            <span className="nav-icon">◈</span>
            Overview
          </button>

          <button
            className={`nav-button ${activeTab === "intentions" ? "active" : ""}`}
            onClick={() => setActiveTab("intentions")}
          >
            <span className="nav-icon">◎</span>
            Intentions
            <span className="nav-count">{intentions.length}</span>
          </button>

          <button
            className={`nav-button ${activeTab === "tasks" ? "active" : ""}`}
            onClick={() => setActiveTab("tasks")}
          >
            <span className="nav-icon">✓</span>
            Tasks
            <span className="nav-count">{pendingTasks.length}</span>
          </button>

          <button
            className={`nav-button ${activeTab === "planner" ? "active" : ""}`}
            onClick={() => setActiveTab("planner")}
          >
            <span className="nav-icon">▦</span>
            Daily Planner
            {scheduleBlocks.length > 0 && <span className="nav-count">{scheduleBlocks.length}</span>}
          </button>

          <button
            className={`nav-button ${activeTab === "assistant" ? "active" : ""}`}
            onClick={() => setActiveTab("assistant")}
          >
            <span className="nav-icon">⚡</span>
            AI Assistant
          </button>
        </nav>

        <div className="sidebar-footer">
          <div className="status-badge">
            <span className="status-indicator"></span>
            Deterministic DAG Engine Active
          </div>
          <div className="user-profile">
            <div className="user-avatar">{user?.email?.[0]?.toUpperCase() || "U"}</div>
            <div className="user-meta">
              <div className="user-email">{user?.email || "Local User"}</div>
              <div className="user-role">Reality Calibrated</div>
            </div>
          </div>
          <button className="btn btn-secondary btn-sm" onClick={handleLogout}>
            Sign Out
          </button>
        </div>
      </aside>

      {/* Main Panel */}
      <main className="main-panel">
        <header className="topbar">
          <div className="breadcrumbs">
            IntentOS / <span>{activeTab.charAt(0).toUpperCase() + activeTab.slice(1)}</span>
          </div>

          <div className="topbar-actions">
            {/* Panic / Off-Track Button */}
            <button
              className="btn btn-panic btn-sm"
              onClick={() => setShowReconcileModal(true)}
              title="Got delayed or interrupted? Reconcile without guilt."
            >
              ⚡ I'm Off-Track
            </button>

            <button className="btn btn-secondary btn-sm" onClick={() => setShowIntentionModal(true)}>
              + Intention
            </button>

            <button
              className="btn btn-primary btn-sm"
              onClick={() => {
                if (intentions.length === 0) {
                  setNotice({ kind: "error", text: "Create an intention before adding tasks." });
                  setShowIntentionModal(true);
                  return;
                }
                setNewTaskIntentionId(intentions[0].id);
                setShowTaskModal(true);
              }}
            >
              + Task
            </button>
          </div>
        </header>

        <div className="content-area">
          {notice && <div className={`toast-notice ${notice.kind}`}>{notice.text}</div>}

          {/* TAB 1: OVERVIEW */}
          {activeTab === "overview" && (
            <div>
              <div className="page-header">
                <div className="page-eyebrow">Attention & Capacity Engine</div>
                <h1 className="page-title">Make room for what matters.</h1>
                <p className="page-description">
                  IntentOS anchors your life horizons into deterministic execution blocks and calms schedule drift.
                </p>
              </div>

              {/* Time Budget & Capacity Bar (YNAB for Time) */}
              <div className="capacity-card">
                <div className="capacity-header">
                  <div className="capacity-title">
                    <span>⏱ Daily Time Capacity (Available Work Window)</span>
                  </div>
                  <div className="capacity-metric">
                    <strong>{scheduledMinutes} mins</strong> allocated of <strong>{dailyCapacityMinutes} mins</strong>
                  </div>
                </div>

                <div className="capacity-bar-track">
                  <div
                    className={`capacity-bar-fill ${isOverbooked ? "overbooked" : "normal"}`}
                    style={{ width: `${Math.min(capacityPercent, 100)}%` }}
                  />
                </div>

                <div className="capacity-footer">
                  <span>
                    {isOverbooked ? (
                      <strong style={{ color: "var(--accent-rose)" }}>
                        ⚠️ Capacity Deficit: Overbooked by {scheduledMinutes - dailyCapacityMinutes} mins! Cut or defer tasks.
                      </strong>
                    ) : (
                      <span style={{ color: "var(--accent-green)" }}>
                        ✓ Sustainable workload: {dailyCapacityMinutes - scheduledMinutes} mins margin buffer available.
                      </span>
                    )}
                  </span>
                  <span>{capacityPercent}% committed</span>
                </div>
              </div>

              {/* KPI Cards */}
              <div className="kpi-grid">
                <div className="kpi-card">
                  <div className="kpi-header">
                    <span>Core Intentions</span>
                    <span className="kpi-icon indigo">◎</span>
                  </div>
                  <div className="kpi-value">{intentions.length}</div>
                  <div className="kpi-subtext">Strategic life anchors</div>
                </div>

                <div className="kpi-card">
                  <div className="kpi-header">
                    <span>Pending Units</span>
                    <span className="kpi-icon amber">⏳</span>
                  </div>
                  <div className="kpi-value">{pendingTasks.length}</div>
                  <div className="kpi-subtext">{totalTasks} total created</div>
                </div>

                <div className="kpi-card">
                  <div className="kpi-header">
                    <span>Execution Rate</span>
                    <span className="kpi-icon green">✓</span>
                  </div>
                  <div className="kpi-value">
                    {totalTasks > 0 ? `${Math.round((completedTasks / totalTasks) * 100)}%` : "0%"}
                  </div>
                  <div className="kpi-subtext">{completedTasks} completed</div>
                </div>

                <div className="kpi-card">
                  <div className="kpi-header">
                    <span>Timeline Blocks</span>
                    <span className="kpi-icon purple">▦</span>
                  </div>
                  <div className="kpi-value">{scheduleBlocks.length}</div>
                  <div className="kpi-subtext">Active execution slots</div>
                </div>
              </div>

              {/* Next Up Hero Card with Focus Mode trigger */}
              {nextUpTask && (
                <div
                  className="card"
                  style={{
                    background: "linear-gradient(135deg, rgba(99,102,241,0.12), rgba(23,27,44,0.95))",
                    borderColor: "rgba(99,102,241,0.35)",
                  }}
                >
                  <div className="card-header">
                    <span className="badge badge-tag">CRITICAL PATH FOCUS</span>
                    <div style={{ display: "flex", gap: "10px" }}>
                      <button className="btn btn-secondary btn-sm" onClick={() => handleUnstickMe(nextUpTask)}>
                        ⚡ Unstick Me
                      </button>
                      <button className="btn btn-primary btn-sm" onClick={() => launchZenFocus(nextUpTask)}>
                        🎯 Enter Zen Focus
                      </button>
                    </div>
                  </div>

                  <h3 style={{ fontSize: "22px", fontWeight: 700, marginBottom: "8px" }}>{nextUpTask.title}</h3>
                  {nextUpTask.description && (
                    <p style={{ color: "var(--text-muted)", marginBottom: "14px" }}>{nextUpTask.description}</p>
                  )}

                  <div className="task-badges">
                    <span className={`badge ${getPriorityLabel(nextUpTask.priority).cls}`}>
                      {getPriorityLabel(nextUpTask.priority).text} Priority
                    </span>
                    <span className={`badge ${getEnergyBadge(nextUpTask.energy_level).cls}`}>
                      {getEnergyBadge(nextUpTask.energy_level).text}
                    </span>
                    <span className="badge badge-duration">⏱ {nextUpTask.estimated_duration} mins</span>
                    {nextUpTask.deadline && (
                      <span className="badge badge-deadline">📅 Due {formatDate(nextUpTask.deadline)}</span>
                    )}
                    {nextUpTask.intention_title && (
                      <span className="badge badge-tag">◎ {nextUpTask.intention_title}</span>
                    )}
                  </div>
                </div>
              )}

              {/* 2-Column Section */}
              <div className="grid-2col">
                {/* Life Stack / Horizon Breakdown */}
                <div className="card">
                  <div className="card-header">
                    <h3 className="card-title">Life Stack Horizons</h3>
                  </div>
                  <p style={{ fontSize: "12px", color: "var(--text-dim)", marginBottom: "16px" }}>
                    How your strategic attention is distributed across core life pillars:
                  </p>

                  <div style={{ display: "flex", flexDirection: "column", gap: "14px" }}>
                    {["Mastery", "Career", "Vitality", "Ventures"].map(horizon => {
                      const count = intentions.filter(i => (i.horizon || "Mastery") === horizon).length;
                      const percent = intentions.length > 0 ? Math.round((count / intentions.length) * 100) : 0;

                      return (
                        <div key={horizon}>
                          <div style={{ display: "flex", justifyContent: "space-between", fontSize: "12px", marginBottom: "4px" }}>
                            <span style={{ fontWeight: 600 }}>{horizon}</span>
                            <span style={{ color: "var(--text-dim)" }}>
                              {count} intentions ({percent}%)
                            </span>
                          </div>
                          <div style={{ height: "6px", background: "rgba(255,255,255,0.06)", borderRadius: "999px", overflow: "hidden" }}>
                            <div
                              style={{
                                width: `${percent}%`,
                                height: "100%",
                                background:
                                  horizon === "Mastery"
                                    ? "var(--primary)"
                                    : horizon === "Career"
                                    ? "var(--accent-cyan)"
                                    : horizon === "Vitality"
                                    ? "var(--accent-green)"
                                    : "var(--accent-amber)",
                              }}
                            />
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>

                {/* Scheduled Timeline Preview */}
                <div className="card">
                  <div className="card-header">
                    <h3 className="card-title">Daily Execution Timeline</h3>
                    <button className="btn btn-secondary btn-sm" onClick={() => setActiveTab("planner")}>
                      Open Planner →
                    </button>
                  </div>

                  {scheduleBlocks.length === 0 ? (
                    <div className="empty-state" style={{ padding: "32px 16px" }}>
                      <div className="empty-icon">📅</div>
                      <div className="empty-title">No Active Schedule</div>
                      <p className="empty-desc">Generate an optimal chronological schedule from your pending tasks.</p>
                      <button className="btn btn-primary btn-sm" onClick={() => handleGeneratePlan(false)} disabled={busy}>
                        Generate Daily Plan
                      </button>
                    </div>
                  ) : (
                    <div className="timeline">
                      {scheduleBlocks.slice(0, 4).map(block => (
                        <div key={block.id} className="timeline-item">
                          <div className="timeline-time">{formatTime(block.start_timestamp)}</div>
                          <div className="timeline-dot" />
                          <div className="timeline-card">
                            <div className="timeline-card-title">{block.task_title || "Scheduled Task"}</div>
                            <div className="timeline-card-meta">
                              <span>until {formatTime(block.end_timestamp)}</span>
                              {block.energy_level && (
                                <span className={`badge ${getEnergyBadge(block.energy_level).cls}`}>
                                  {getEnergyBadge(block.energy_level).text}
                                </span>
                              )}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* TAB 2: INTENTIONS */}
          {activeTab === "intentions" && (
            <div>
              <div className="page-header" style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                <div>
                  <div className="page-eyebrow">Strategic Life Anchors</div>
                  <h1 className="page-title">Intentions</h1>
                  <p className="page-description">
                    High-level outcomes connected to core horizons that give purpose to your daily tasks.
                  </p>
                </div>
                <button className="btn btn-primary" onClick={() => setShowIntentionModal(true)}>
                  + New Intention
                </button>
              </div>

              {/* Horizon Filter pills */}
              <div className="filter-bar">
                {["all", "Mastery", "Career", "Vitality", "Ventures"].map(h => (
                  <button
                    key={h}
                    className={`filter-btn ${selectedHorizonFilter === h ? "active" : ""}`}
                    onClick={() => setSelectedHorizonFilter(h)}
                  >
                    {h === "all" ? "All Horizons" : h}
                  </button>
                ))}
              </div>

              {intentions.length === 0 ? (
                <div className="empty-state">
                  <div className="empty-icon">◎</div>
                  <div className="empty-title">No intentions anchored yet</div>
                  <p className="empty-desc">Intentions give purpose to your daily actions. Define your first goal.</p>
                  <button className="btn btn-primary" onClick={() => setShowIntentionModal(true)}>
                    Anchor First Intention
                  </button>
                </div>
              ) : (
                <div className="intentions-grid">
                  {intentions
                    .filter(i => selectedHorizonFilter === "all" || (i.horizon || "Mastery") === selectedHorizonFilter)
                    .map(i => {
                      const linkedTasks = tasks.filter(t => t.intention_id === i.id);
                      const completedCount = linkedTasks.filter(t => t.status === "completed").length;

                      return (
                        <div key={i.id} className="intention-card">
                          <div className="intention-header">
                            <div>
                              <span className="badge badge-horizon" style={{ marginBottom: "6px" }}>
                                {i.horizon || "Mastery"}
                              </span>
                              <h3 className="intention-title">{i.title}</h3>
                            </div>
                            <button
                              className="btn btn-danger btn-sm"
                              title="Delete intention"
                              onClick={e => {
                                e.stopPropagation();
                                handleDeleteIntention(i.id, i.title);
                              }}
                            >
                              ✕
                            </button>
                          </div>

                          {i.description && <p className="intention-desc">{i.description}</p>}

                          <div className="intention-footer">
                            <span>
                              {linkedTasks.length} tasks ({completedCount} done)
                            </span>
                            <button
                              className="btn btn-secondary btn-sm"
                              onClick={() => {
                                setSelectedIntentionFilter(i.id);
                                setActiveTab("tasks");
                              }}
                            >
                              View Tasks →
                            </button>
                          </div>
                        </div>
                      );
                    })}
                </div>
              )}
            </div>
          )}

          {/* TAB 3: TASKS */}
          {activeTab === "tasks" && (
            <div>
              <div className="page-header" style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                <div>
                  <div className="page-eyebrow">Atomic Action Units</div>
                  <h1 className="page-title">Tasks & Energy</h1>
                  <p className="page-description">
                    Manage actionable units, energy requirements, and actual durations.
                  </p>
                </div>
                <button
                  className="btn btn-primary"
                  onClick={() => {
                    if (intentions.length === 0) {
                      setNotice({ kind: "error", text: "Create an intention before adding tasks." });
                      setShowIntentionModal(true);
                      return;
                    }
                    setNewTaskIntentionId(intentions[0].id);
                    setShowTaskModal(true);
                  }}
                >
                  + Add Task
                </button>
              </div>

              {/* Filters */}
              <div className="filter-bar">
                <span style={{ fontSize: "12px", color: "var(--text-dim)", fontWeight: 600 }}>Filter by Intention:</span>
                <select
                  className="form-select"
                  value={selectedIntentionFilter}
                  onChange={e => setSelectedIntentionFilter(e.target.value)}
                  style={{ minWidth: "160px" }}
                >
                  <option value="all">All Intentions ({intentions.length})</option>
                  {intentions.map(i => (
                    <option key={i.id} value={i.id}>
                      {i.title}
                    </option>
                  ))}
                </select>

                <div style={{ marginLeft: "auto", display: "flex", gap: "6px" }}>
                  <button
                    className={`filter-btn ${taskStatusFilter === "all" ? "active" : ""}`}
                    onClick={() => setTaskStatusFilter("all")}
                  >
                    All ({tasks.length})
                  </button>
                  <button
                    className={`filter-btn ${taskStatusFilter === "pending" ? "active" : ""}`}
                    onClick={() => setTaskStatusFilter("pending")}
                  >
                    Pending ({pendingTasks.length})
                  </button>
                  <button
                    className={`filter-btn ${taskStatusFilter === "completed" ? "active" : ""}`}
                    onClick={() => setTaskStatusFilter("completed")}
                  >
                    Completed ({completedTasks})
                  </button>
                </div>
              </div>

              {/* Task list */}
              {filteredTasks.length === 0 ? (
                <div className="empty-state">
                  <div className="empty-icon">✓</div>
                  <div className="empty-title">No tasks found</div>
                  <p className="empty-desc">No tasks match your selected filter criteria.</p>
                  <button
                    className="btn btn-primary"
                    onClick={() => {
                      if (intentions.length === 0) {
                        setShowIntentionModal(true);
                        return;
                      }
                      setNewTaskIntentionId(intentions[0].id);
                      setShowTaskModal(true);
                    }}
                  >
                    Create a Task
                  </button>
                </div>
              ) : (
                <div className="task-list">
                  {filteredTasks.map(t => {
                    const isDone = t.status === "completed";
                    const priority = getPriorityLabel(t.priority);
                    const energy = getEnergyBadge(t.energy_level);

                    return (
                      <div key={t.id} className={`task-item ${isDone ? "completed" : ""}`}>
                        <button
                          className={`task-checkbox ${isDone ? "checked" : ""}`}
                          onClick={() => handleToggleTaskStatus(t)}
                          title={isDone ? "Mark pending" : "Mark completed"}
                        >
                          {isDone ? "✓" : ""}
                        </button>

                        <div className="task-info">
                          <div className="task-item-title">{t.title}</div>
                          {t.description && (
                            <p style={{ fontSize: "12px", color: "var(--text-muted)", marginBottom: "4px" }}>
                              {t.description}
                            </p>
                          )}
                          <div className="task-badges">
                            <span className={`badge ${priority.cls}`}>{priority.text}</span>
                            <span className={`badge ${energy.cls}`}>{energy.text}</span>
                            <span className="badge badge-duration">⏱ {t.estimated_duration} mins</span>
                            {isDone && t.actual_duration > 0 && (
                              <span className="badge badge-tag" style={{ background: "rgba(16,185,129,0.15)", color: "#34d399" }}>
                                Actual: {t.actual_duration}m
                              </span>
                            )}
                            {t.deadline && <span className="badge badge-deadline">📅 Due {formatDate(t.deadline)}</span>}
                            {t.intention_title && <span className="badge badge-tag">◎ {t.intention_title}</span>}
                          </div>
                        </div>

                        <div style={{ display: "flex", gap: "6px" }}>
                          {!isDone && (
                            <>
                              <button
                                className="btn btn-secondary btn-sm"
                                style={{ padding: "4px 8px" }}
                                onClick={() => handleUnstickMe(t)}
                                title="Decompose into 5-minute activation steps"
                              >
                                ⚡ Unstick
                              </button>
                              <button
                                className="btn btn-primary btn-sm"
                                style={{ padding: "4px 8px" }}
                                onClick={() => launchZenFocus(t)}
                                title="Enter Zen Focus"
                              >
                                🎯 Focus
                              </button>
                            </>
                          )}
                          <button
                            className="btn btn-danger btn-sm"
                            style={{ padding: "4px 8px" }}
                            onClick={() => handleDeleteTask(t.id)}
                            title="Delete task"
                          >
                            ✕
                          </button>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          )}

          {/* TAB 4: PLANNER */}
          {activeTab === "planner" && (
            <div>
              <div className="page-header" style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                <div>
                  <div className="page-eyebrow">Deterministic Topological Engine</div>
                  <h1 className="page-title">Daily Execution Plan</h1>
                  <p className="page-description">
                    Optimal task ordering with zero-guilt dynamic reality adaptation.
                  </p>
                </div>

                <div style={{ display: "flex", gap: "10px" }}>
                  <button className="btn btn-panic" onClick={() => setShowReconcileModal(true)}>
                    ⚡ I'm Off-Track
                  </button>
                  <button className="btn btn-secondary" onClick={() => handleGeneratePlan(true)} disabled={busy}>
                    🔄 Replan
                  </button>
                  <button className="btn btn-primary" onClick={() => handleGeneratePlan(false)} disabled={busy}>
                    ⚡ Generate Plan
                  </button>
                </div>
              </div>

              {/* Warning alert if deadlines exceeded */}
              {unscheduledWarnings.length > 0 && (
                <div className="toast-notice error" style={{ flexDirection: "column", alignItems: "flex-start" }}>
                  <strong>Scheduling Notices & Warnings:</strong>
                  <ul style={{ paddingLeft: "18px", marginTop: "4px" }}>
                    {unscheduledWarnings.map((w, idx) => (
                      <li key={idx}>{w}</li>
                    ))}
                  </ul>
                </div>
              )}

              <div className="grid-2col">
                {/* Timeline Column */}
                <div className="card">
                  <div className="card-header">
                    <h3 className="card-title">Schedule Blocks</h3>
                    <span style={{ fontSize: "12px", color: "var(--text-dim)" }}>
                      {scheduleBlocks.length} slots allocated ({scheduledMinutes} mins)
                    </span>
                  </div>

                  {scheduleBlocks.length === 0 ? (
                    <div className="empty-state">
                      <div className="empty-icon">▦</div>
                      <div className="empty-title">No Schedule Generated</div>
                      <p className="empty-desc">
                        Click "Generate Plan" to calculate an optimal chronological timeline for all your pending tasks.
                      </p>
                      <button className="btn btn-primary" onClick={() => handleGeneratePlan(false)} disabled={busy}>
                        Generate Plan Now
                      </button>
                    </div>
                  ) : (
                    <div className="timeline">
                      {scheduleBlocks.map(block => (
                        <div key={block.id} className="timeline-item">
                          <div className="timeline-time">{formatTime(block.start_timestamp)}</div>
                          <div className="timeline-dot" />
                          <div className="timeline-card">
                            <div className="timeline-card-title">{block.task_title || "Task Execution Slot"}</div>
                            <div className="timeline-card-meta">
                              <span>
                                {formatTime(block.start_timestamp)} — {formatTime(block.end_timestamp)}
                              </span>
                              {block.energy_level && (
                                <span className={`badge ${getEnergyBadge(block.energy_level).cls}`}>
                                  {getEnergyBadge(block.energy_level).text}
                                </span>
                              )}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                {/* Plan Revisions Column */}
                <div className="card">
                  <div className="card-header">
                    <h3 className="card-title">Adaptive Plan Revisions</h3>
                  </div>

                  {planHistory.length === 0 ? (
                    <p style={{ color: "var(--text-dim)", fontSize: "13px" }}>
                      No plan revisions saved yet. Revisions are created whenever a plan is generated or reality is reconciled.
                    </p>
                  ) : (
                    <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
                      {planHistory.map(rev => (
                        <div
                          key={rev.id}
                          style={{
                            padding: "12px 14px",
                            background: "rgba(255,255,255,0.03)",
                            border: "1px solid var(--border-subtle)",
                            borderRadius: "var(--radius-sm)",
                          }}
                        >
                          <div style={{ fontWeight: 600, fontSize: "13px" }}>{rev.description}</div>
                          <div style={{ fontSize: "11px", color: "var(--text-dim)", marginTop: "4px" }}>
                            {new Date(rev.created_at).toLocaleString()}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* TAB 5: AI ASSISTANT */}
          {activeTab === "assistant" && (
            <div>
              <div className="page-header">
                <div className="page-eyebrow">Natural Language Intent Decomposer</div>
                <h1 className="page-title">AI Planning Assistant</h1>
                <p className="page-description">
                  Express messy human goals. IntentOS structures intentions, durations, and cognitive energy levels.
                </p>
              </div>

              <div className="card" style={{ maxWidth: "800px" }}>
                <form onSubmit={handleAssistantSubmit}>
                  <div className="form-group">
                    <label className="form-label">Express your intention in plain language</label>
                    <textarea
                      className="form-textarea"
                      rows={5}
                      value={assistantPrompt}
                      onChange={e => setAssistantPrompt(e.target.value)}
                      placeholder="e.g. I have a Computer Networks exam on Friday. I need to finish routing protocols (60m, high energy), revise subnetting (45m), and solve a practice paper (90m)."
                    />
                  </div>

                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: "16px" }}>
                    <span style={{ fontSize: "12px", color: "var(--text-dim)" }}>
                      Automatically extracts intentions, tasks, durations, and energy loads.
                    </span>
                    <button type="submit" className="btn btn-primary" disabled={busy}>
                      {busy ? "Parsing & Scheduling..." : "Extract & Schedule →"}
                    </button>
                  </div>
                </form>
              </div>
            </div>
          )}
        </div>
      </main>

      {/* MODAL: CREATE INTENTION */}
      {showIntentionModal && (
        <div className="modal-overlay" onClick={() => setShowIntentionModal(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Anchor New Intention</h3>
              <button className="modal-close" onClick={() => setShowIntentionModal(false)}>
                ✕
              </button>
            </div>
            <form onSubmit={handleCreateIntention}>
              <div className="form-group">
                <label className="form-label">Life Stack Horizon *</label>
                <select
                  className="form-select"
                  value={newIntentionHorizon}
                  onChange={e => setNewIntentionHorizon(e.target.value)}
                >
                  <option value="Mastery">🎓 Mastery & Learning</option>
                  <option value="Career">💻 Career & Engineering</option>
                  <option value="Vitality">🏃 Vitality & Health</option>
                  <option value="Ventures">🚀 Ventures & Impact</option>
                </select>
              </div>

              <div className="form-group">
                <label className="form-label">Intention Title *</label>
                <input
                  className="form-input"
                  required
                  placeholder="e.g. Pass Computer Networks with an A"
                  value={newIntentionTitle}
                  onChange={e => setNewIntentionTitle(e.target.value)}
                />
              </div>

              <div className="form-group">
                <label className="form-label">Description (Optional)</label>
                <textarea
                  className="form-textarea"
                  placeholder="Why does this matter? What does success look like?"
                  value={newIntentionDesc}
                  onChange={e => setNewIntentionDesc(e.target.value)}
                />
              </div>

              <div className="modal-actions">
                <button type="button" className="btn btn-secondary" onClick={() => setShowIntentionModal(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary" disabled={busy}>
                  Anchor Intention
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: CREATE TASK */}
      {showTaskModal && (
        <div className="modal-overlay" onClick={() => setShowTaskModal(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Add Actionable Task</h3>
              <button className="modal-close" onClick={() => setShowTaskModal(false)}>
                ✕
              </button>
            </div>
            <form onSubmit={handleCreateTask}>
              <div className="form-group">
                <label className="form-label">Associated Intention *</label>
                <select
                  className="form-select"
                  required
                  value={newTaskIntentionId}
                  onChange={e => setNewTaskIntentionId(e.target.value)}
                >
                  {intentions.map(i => (
                    <option key={i.id} value={i.id}>
                      {i.title} ({i.horizon || "Mastery"})
                    </option>
                  ))}
                </select>
              </div>

              <div className="form-group">
                <label className="form-label">Task Title *</label>
                <input
                  className="form-input"
                  required
                  placeholder="e.g. Solve Practice Paper 2"
                  value={newTaskTitle}
                  onChange={e => setNewTaskTitle(e.target.value)}
                />
              </div>

              <div className="form-group">
                <label className="form-label">Description (Optional)</label>
                <textarea
                  className="form-textarea"
                  placeholder="Specific focus or instructions"
                  value={newTaskDesc}
                  onChange={e => setNewTaskDesc(e.target.value)}
                />
              </div>

              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">Duration (Minutes)</label>
                  <input
                    className="form-input"
                    type="number"
                    min={5}
                    step={5}
                    value={newTaskDuration}
                    onChange={e => setNewTaskDuration(parseInt(e.target.value, 10) || 30)}
                  />
                </div>

                <div className="form-group">
                  <label className="form-label">Cognitive Energy</label>
                  <select
                    className="form-select"
                    value={newTaskEnergy}
                    onChange={e => setNewTaskEnergy(e.target.value)}
                  >
                    <option value="high">🧠 Deep Work (High Focus)</option>
                    <option value="medium">⚡ Medium Energy</option>
                    <option value="low">☕ Low Energy (Admin/Review)</option>
                  </select>
                </div>
              </div>

              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">Priority</label>
                  <select
                    className="form-select"
                    value={newTaskPriority}
                    onChange={e => setNewTaskPriority(parseInt(e.target.value, 10))}
                  >
                    <option value={0}>Low</option>
                    <option value={1}>Medium</option>
                    <option value={2}>High</option>
                  </select>
                </div>

                <div className="form-group">
                  <label className="form-label">Deadline (Optional)</label>
                  <input
                    className="form-input"
                    type="datetime-local"
                    value={newTaskDeadline}
                    onChange={e => setNewTaskDeadline(e.target.value)}
                  />
                </div>
              </div>

              <div className="modal-actions">
                <button type="button" className="btn btn-secondary" onClick={() => setShowTaskModal(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary" disabled={busy}>
                  Queue Task
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: REALITY RECONCILIATION (I'M OFF-TRACK) */}
      {showReconcileModal && (
        <div className="modal-overlay" onClick={() => setShowReconcileModal(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">⚡ Reconcile Reality</h3>
              <button className="modal-close" onClick={() => setShowReconcileModal(false)}>
                ✕
              </button>
            </div>
            <p style={{ fontSize: "13px", color: "var(--text-muted)", marginBottom: "16px" }}>
              Life happened? Don't stress or drag cards around. IntentOS recalculates your schedule blocks while defending critical path deadlines.
            </p>

            <form onSubmit={handleReconcileReality}>
              <div className="form-group">
                <label className="form-label">How much time do you need before restarting?</label>
                <div style={{ display: "flex", gap: "8px", marginTop: "4px" }}>
                  {[15, 30, 45, 60, 90].map(mins => (
                    <button
                      type="button"
                      key={mins}
                      className={`filter-btn ${delayMinutes === mins ? "active" : ""}`}
                      onClick={() => setDelayMinutes(mins)}
                    >
                      +{mins}m
                    </button>
                  ))}
                </div>
              </div>

              <div className="form-group">
                <label className="form-label">What happened?</label>
                <select
                  className="form-select"
                  value={delayReason}
                  onChange={e => setDelayReason(e.target.value)}
                >
                  <option value="Task took longer than anticipated">Previous task took longer</option>
                  <option value="Overslept / Started work late">Started work late</option>
                  <option value="Unexpected interruption / Meeting">Interruption / Emergency</option>
                  <option value="Low energy / Taking a breather">Low energy / Taking a break</option>
                </select>
              </div>

              <div className="modal-actions">
                <button type="button" className="btn btn-secondary" onClick={() => setShowReconcileModal(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-panic" disabled={busy}>
                  ⚡ Reconcile Schedule
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: UNSTICK ME */}
      {unstickTask && (
        <div className="modal-overlay" onClick={() => setUnstickTask(null)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">⚡ Unstick: {unstickTask.title}</h3>
              <button className="modal-close" onClick={() => setUnstickTask(null)}>
                ✕
              </button>
            </div>
            <p style={{ fontSize: "13px", color: "var(--text-muted)", marginBottom: "16px" }}>
              Procrastination is usually caused by an overwhelming task scope. Here are 3 non-threatening 5-minute activation steps:
            </p>

            <div style={{ display: "flex", flexDirection: "column", gap: "10px", marginBottom: "20px" }}>
              {unstickSteps.map((step, idx) => (
                <div
                  key={idx}
                  style={{
                    padding: "12px 14px",
                    background: "rgba(99,102,241,0.08)",
                    border: "1px solid rgba(99,102,241,0.2)",
                    borderRadius: "var(--radius-sm)",
                    fontSize: "13px",
                    lineHeight: "1.4",
                  }}
                >
                  {step}
                </div>
              ))}
            </div>

            <div className="modal-actions">
              <button
                type="button"
                className="btn btn-primary"
                onClick={() => {
                  launchZenFocus(unstickTask);
                  setUnstickTask(null);
                }}
              >
                🎯 Start Step 1 in Zen Focus
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
