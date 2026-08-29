"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { api, sessionUser } from "../../lib/api";
import { priorityLabel, roleLabel, stationLabel, statusLabel, timeOffTypeLabel } from "../../lib/labels";
import { isManagerRole } from "../../lib/roles";
import { activeAttendance, addDaysISO, formatDate, formatTime, isoDate, stations } from "../../lib/workforce";

const emptySummary = { shifts_today: 0, on_duty: 0, pending_time_off: 0, open_tasks: 0 };

function Pill({ children, tone = "neutral" }) {
  return <span className={`ops-pill ops-pill-${tone}`}>{children}</span>;
}

function Metric({ label, value, hint }) {
  return (
    <article className="ops-metric-card">
      <p className="eyebrow">{label}</p>
      <p className="mt-3 text-4xl font-black tracking-tight text-emerald-950">{value}</p>
      <p className="mt-2 text-xs font-semibold text-slate-500">{hint}</p>
    </article>
  );
}

function Empty({ children }) {
  return <div className="ops-empty">{children}</div>;
}

export default function WorkforcePage() {
  const user = sessionUser();
  const manager = isManagerRole(user?.role);
  const today = isoDate();
  const [summary, setSummary] = useState(emptySummary);
  const [shifts, setShifts] = useState([]);
  const [attendance, setAttendance] = useState([]);
  const [timeOff, setTimeOff] = useState([]);
  const [tasks, setTasks] = useState([]);
  const [users, setUsers] = useState([]);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const [leaveForm, setLeaveForm] = useState({ start_date: today, end_date: today, type: "permission", reason: "" });
  const [shiftForm, setShiftForm] = useState({ employee_id: "", shift_date: today, start_time: "11:00", end_time: "19:00", station: "service", notes: "" });
  const [taskForm, setTaskForm] = useState({ shift_date: today, title: "", station: "service", assigned_to_id: "0", priority: "normal" });

  const from = today;
  const to = addDaysISO(7);

  const load = useCallback(async () => {
    setError("");
    try {
      const calls = [
        api("/api/workforce/summary"),
        api(`/api/workforce/shifts?from=${from}&to=${to}`),
        api(`/api/workforce/attendance?from=${from}&to=${to}`),
        api("/api/workforce/time-off"),
        api(`/api/workforce/tasks?from=${from}&to=${to}`),
      ];
      if (manager) calls.push(api("/api/users"));
      const [nextSummary, nextShifts, nextAttendance, nextTimeOff, nextTasks, nextUsers = []] = await Promise.all(calls);
      setSummary(nextSummary || emptySummary);
      setShifts(nextShifts || []);
      setAttendance(nextAttendance || []);
      setTimeOff(nextTimeOff || []);
      setTasks(nextTasks || []);
      setUsers((nextUsers || []).filter((row) => row.active && row.role !== "admin"));
    } catch (e) {
      setError(e.message);
    }
  }, [from, manager, to]);

  useEffect(() => { load(); }, [load]);

  const currentAttendance = useMemo(() => activeAttendance(attendance), [attendance]);
  const todayShift = useMemo(() => shifts.find((row) => row.shift_date === today) || null, [shifts, today]);
  const todayTasks = useMemo(() => tasks.filter((row) => row.shift_date === today), [tasks, today]);
  const pendingRequests = useMemo(() => timeOff.filter((row) => row.status === "pending"), [timeOff]);

  async function action(name, fn, success) {
    setBusy(name);
    setError("");
    setNotice("");
    try {
      await fn();
      setNotice(success);
      await load();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy("");
    }
  }

  function selectedUser(id) {
    return users.find((row) => String(row.id) === String(id));
  }

  async function submitLeave(event) {
    event.preventDefault();
    await action("leave", () => api("/api/workforce/time-off", { method: "POST", body: JSON.stringify(leaveForm) }), "Pengajuan terkirim ke PIC/owner.");
    setLeaveForm((value) => ({ ...value, reason: "" }));
  }

  async function submitShift(event) {
    event.preventDefault();
    const employee = selectedUser(shiftForm.employee_id);
    if (!employee) {
      setError("Pilih anggota tim untuk dijadwalkan.");
      return;
    }
    const payload = { ...shiftForm, employee_id: Number(employee.id), employee_name: employee.name };
    await action("shift", () => api("/api/workforce/shifts", { method: "POST", body: JSON.stringify(payload) }), "Shift baru berhasil diterbitkan.");
  }

  async function submitTask(event) {
    event.preventDefault();
    const employee = selectedUser(taskForm.assigned_to_id);
    const payload = {
      ...taskForm,
      assigned_to_id: Number(taskForm.assigned_to_id || 0),
      assigned_to_name: employee?.name || "Semua Tim",
    };
    await action("task", () => api("/api/workforce/tasks", { method: "POST", body: JSON.stringify(payload) }), "Checklist shift berhasil ditambahkan.");
    setTaskForm((value) => ({ ...value, title: "" }));
  }

  return (
    <main className="page-wrap">
      <section className="ops-hero">
        <div className="relative z-10 grid gap-8 lg:grid-cols-[1.2fr_.8fr] lg:items-end">
          <div>
            <span className="hero-badge">Tropical Steak House · People & Shift Ops</span>
            <h1 className="mt-5 max-w-3xl text-4xl font-black tracking-[-.05em] text-white md:text-6xl">
              {manager ? "Tim siap sebelum tamu datang." : `Hari kerja ${user?.name || "Anda"}, lebih jelas.`}
            </h1>
            <p className="mt-5 max-w-2xl text-sm leading-7 text-emerald-50/75">
              {manager
                ? "Atur coverage station, kehadiran, izin, dan checklist shift dari satu ruang kerja agar kitchen dan service bergerak dengan ritme yang sama."
                : "Lihat shift, clock in/out, checklist kerja, dan pengajuan izin tanpa harus menunggu informasi dari grup chat."}
            </p>
          </div>
          <div className="ops-role-card">
            <p className="text-[10px] font-black uppercase tracking-[.18em] text-amber-200">Mode akses</p>
            <p className="mt-2 text-2xl font-black text-white">{roleLabel(user?.role)}</p>
            <p className="mt-2 text-xs leading-5 text-emerald-50/65">
              {manager ? "Kontrol operasional tim dan approval tersedia." : "Hanya data kerja pribadi dan tugas yang relevan yang ditampilkan."}
            </p>
          </div>
        </div>
      </section>

      {error && <div className="toast-error mt-5">{error}</div>}
      {notice && <div className="toast-success mt-5">{notice}</div>}

      <section className="mt-5 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Metric label={manager ? "Shift Hari Ini" : "Shift Saya"} value={summary.shifts_today} hint={manager ? "coverage yang terjadwal" : todayShift ? `${todayShift.start_time}–${todayShift.end_time} · ${stationLabel(todayShift.station)}` : "belum ada jadwal hari ini"} />
        <Metric label={manager ? "Sedang Bertugas" : "Status Kehadiran"} value={summary.on_duty} hint={manager ? "anggota tim belum clock out" : currentAttendance ? `Masuk ${formatTime(currentAttendance.clock_in)}` : "belum clock in"} />
        <Metric label="Menunggu Persetujuan" value={summary.pending_time_off} hint={manager ? "izin/cuti perlu keputusan" : "pengajuan pribadi aktif"} />
        <Metric label="Checklist Terbuka" value={summary.open_tasks} hint="tugas shift yang belum selesai" />
      </section>

      <section className="mt-5 grid gap-5 xl:grid-cols-[1.15fr_.85fr]">
        <div className="card p-6 md:p-7">
          <div className="flex flex-wrap items-end justify-between gap-4">
            <div>
              <p className="eyebrow">{manager ? "Coverage 7 Hari" : "Jadwal Saya"}</p>
              <h2 className="mt-2 text-2xl font-black text-emerald-950">Shift yang akan datang</h2>
            </div>
            {!manager && (
              <div className="flex gap-2">
                <button type="button" className="btn" disabled={busy === "clock" || currentAttendance || !todayShift} onClick={() => action("clock", () => api("/api/workforce/attendance/clock-in", { method: "POST" }), "Clock in tercatat.")}>Clock In</button>
                <button type="button" className="btn btn-secondary" disabled={busy === "clock" || !currentAttendance} onClick={() => action("clock", () => api("/api/workforce/attendance/clock-out", { method: "POST" }), "Clock out tercatat.")}>Clock Out</button>
              </div>
            )}
          </div>

          <div className="mt-6 overflow-x-auto">
            {shifts.length ? (
              <table className="ops-table">
                <thead><tr><th>Tanggal</th>{manager && <th>Tim</th>}<th>Jam</th><th>Station</th><th>Status</th></tr></thead>
                <tbody>
                  {shifts.map((row) => (
                    <tr key={row.id}>
                      <td className="font-bold text-emerald-950">{formatDate(row.shift_date)}</td>
                      {manager && <td>{row.employee_name}</td>}
                      <td>{row.start_time}–{row.end_time}</td>
                      <td><Pill>{stationLabel(row.station)}</Pill></td>
                      <td><Pill tone="green">{statusLabel(row.status)}</Pill></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : <Empty>Belum ada shift pada rentang tujuh hari ke depan.</Empty>}
          </div>
        </div>

        <div className="card p-6 md:p-7">
          <p className="eyebrow">Shift Checklist</p>
          <h2 className="mt-2 text-2xl font-black text-emerald-950">Fokus hari ini</h2>
          <div className="mt-5 space-y-3">
            {todayTasks.length ? todayTasks.map((item) => (
              <div className={`ops-task ${item.status === "done" ? "done" : ""}`} key={item.id}>
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <Pill tone={item.priority === "high" ? "amber" : "neutral"}>{priorityLabel(item.priority)}</Pill>
                    <Pill>{stationLabel(item.station)}</Pill>
                  </div>
                  <p className="mt-2 font-black text-emerald-950">{item.title}</p>
                  <p className="mt-1 text-xs text-slate-500">{item.assigned_to_name || "Semua Tim"}</p>
                </div>
                <button className="task-check" type="button" aria-label={`Ubah status ${item.title}`} disabled={busy === `task-${item.id}`} onClick={() => action(`task-${item.id}`, () => api("/api/workforce/tasks", { method: "PATCH", body: JSON.stringify({ id: item.id, status: item.status === "done" ? "open" : "done" }) }), "Status checklist diperbarui.")}>{item.status === "done" ? "✓" : "○"}</button>
              </div>
            )) : <Empty>Checklist hari ini masih kosong.</Empty>}
          </div>
        </div>
      </section>

      {manager ? (
        <ManagerWorkspace
          users={users}
          shiftForm={shiftForm}
          setShiftForm={setShiftForm}
          taskForm={taskForm}
          setTaskForm={setTaskForm}
          submitShift={submitShift}
          submitTask={submitTask}
          busy={busy}
          attendance={attendance}
          pendingRequests={pendingRequests}
          review={(id, status) => action(`review-${id}`, () => api("/api/workforce/time-off", { method: "PATCH", body: JSON.stringify({ id, status, review_note: status === "approved" ? "Coverage shift terkonfirmasi." : "Perlu penjadwalan ulang." }) }), `Pengajuan ${status === "approved" ? "disetujui" : "ditolak"}.`)}
          canManageAccess={user?.role === "admin"}
        />
      ) : (
        <EmployeeWorkspace
          leaveForm={leaveForm}
          setLeaveForm={setLeaveForm}
          submitLeave={submitLeave}
          busy={busy}
          timeOff={timeOff}
          attendance={attendance}
        />
      )}
    </main>
  );
}

function ManagerWorkspace({ users, shiftForm, setShiftForm, taskForm, setTaskForm, submitShift, submitTask, busy, attendance, pendingRequests, review, canManageAccess }) {
  return (
    <section className="mt-5 grid gap-5 xl:grid-cols-2">
      <div className="space-y-5">
        <form className="card p-6 md:p-7" onSubmit={submitShift}>
          <p className="eyebrow">Schedule Builder</p>
          <h2 className="mt-2 text-xl font-black text-emerald-950">Terbitkan shift</h2>
          <div className="mt-5 grid gap-3 sm:grid-cols-2">
            <select className="input sm:col-span-2" value={shiftForm.employee_id} onChange={(e) => setShiftForm({ ...shiftForm, employee_id: e.target.value })} required>
              <option value="">Pilih anggota tim</option>
              {users.map((row) => <option key={row.id} value={row.id}>{row.name} · {roleLabel(row.role)}</option>)}
            </select>
            <input className="input" type="date" value={shiftForm.shift_date} onChange={(e) => setShiftForm({ ...shiftForm, shift_date: e.target.value })} required />
            <select className="input" value={shiftForm.station} onChange={(e) => setShiftForm({ ...shiftForm, station: e.target.value })}>{stations.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select>
            <input className="input" type="time" value={shiftForm.start_time} onChange={(e) => setShiftForm({ ...shiftForm, start_time: e.target.value })} required />
            <input className="input" type="time" value={shiftForm.end_time} onChange={(e) => setShiftForm({ ...shiftForm, end_time: e.target.value })} required />
            <input className="input sm:col-span-2" placeholder="Catatan shift, mis. closing / event" value={shiftForm.notes} onChange={(e) => setShiftForm({ ...shiftForm, notes: e.target.value })} />
          </div>
          <button className="btn mt-4 w-full" disabled={busy === "shift"}>Terbitkan Shift</button>
        </form>

        <form className="card p-6 md:p-7" onSubmit={submitTask}>
          <p className="eyebrow">Standard Work</p>
          <h2 className="mt-2 text-xl font-black text-emerald-950">Tambahkan checklist</h2>
          <div className="mt-5 grid gap-3 sm:grid-cols-2">
            <input className="input sm:col-span-2" placeholder="Contoh: cek suhu chiller sebelum service" value={taskForm.title} onChange={(e) => setTaskForm({ ...taskForm, title: e.target.value })} required />
            <input className="input" type="date" value={taskForm.shift_date} onChange={(e) => setTaskForm({ ...taskForm, shift_date: e.target.value })} required />
            <select className="input" value={taskForm.station} onChange={(e) => setTaskForm({ ...taskForm, station: e.target.value })}>{stations.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select>
            <select className="input" value={taskForm.assigned_to_id} onChange={(e) => setTaskForm({ ...taskForm, assigned_to_id: e.target.value })}>
              <option value="0">Semua Tim</option>
              {users.map((row) => <option key={row.id} value={row.id}>{row.name}</option>)}
            </select>
            <select className="input" value={taskForm.priority} onChange={(e) => setTaskForm({ ...taskForm, priority: e.target.value })}>
              <option value="normal">Prioritas normal</option><option value="high">Prioritas tinggi</option><option value="low">Prioritas rendah</option>
            </select>
          </div>
          <button className="btn mt-4 w-full" disabled={busy === "task"}>Tambahkan Checklist</button>
        </form>
      </div>

      <div className="space-y-5">
        <div className="card p-6 md:p-7">
          <div className="flex items-end justify-between gap-3"><div><p className="eyebrow">Approval Queue</p><h2 className="mt-2 text-xl font-black text-emerald-950">Izin & cuti</h2></div><Pill tone="amber">{pendingRequests.length} pending</Pill></div>
          <div className="mt-5 space-y-3">
            {pendingRequests.length ? pendingRequests.map((row) => (
              <article key={row.id} className="ops-request">
                <div className="flex flex-wrap items-start justify-between gap-3"><div><p className="font-black text-emerald-950">{row.employee_name}</p><p className="mt-1 text-xs text-slate-500">{timeOffTypeLabel(row.type)} · {formatDate(row.start_date)} sampai {formatDate(row.end_date)}</p></div><Pill tone="amber">Menunggu</Pill></div>
                <p className="mt-3 text-sm text-slate-600">{row.reason}</p>
                <div className="mt-4 flex gap-2"><button type="button" className="btn flex-1" disabled={busy === `review-${row.id}`} onClick={() => review(row.id, "approved")}>Setujui</button><button type="button" className="btn btn-secondary flex-1" disabled={busy === `review-${row.id}`} onClick={() => review(row.id, "rejected")}>Tolak</button></div>
              </article>
            )) : <Empty>Tidak ada pengajuan yang menunggu keputusan.</Empty>}
          </div>
        </div>

        <div className="card p-6 md:p-7">
          <p className="eyebrow">Attendance Pulse</p><h2 className="mt-2 text-xl font-black text-emerald-950">Kehadiran terbaru</h2>
          <div className="mt-5 space-y-3">
            {attendance.length ? attendance.slice(0, 8).map((row) => (
              <div className="flex items-center justify-between gap-4 border-b border-emerald-950/7 pb-3" key={row.id}><div><p className="font-bold text-emerald-950">{row.employee_name}</p><p className="text-xs text-slate-500">{formatDate(row.work_date)} · masuk {formatTime(row.clock_in)}</p></div><Pill tone={row.clock_out ? "neutral" : "green"}>{row.clock_out ? `Pulang ${formatTime(row.clock_out)}` : "On duty"}</Pill></div>
            )) : <Empty>Belum ada aktivitas kehadiran pada rentang ini.</Empty>}
          </div>
          {canManageAccess ? (
            <Link href="/users" className="mt-5 inline-flex text-xs font-black text-amber-700">Kelola akses karyawan →</Link>
          ) : (
            <p className="mt-5 text-xs font-semibold leading-5 text-slate-500">PIC dapat melihat anggota tim untuk scheduling. Perubahan akun dan role tetap khusus Owner.</p>
          )}
        </div>
      </div>
    </section>
  );
}

function EmployeeWorkspace({ leaveForm, setLeaveForm, submitLeave, busy, timeOff, attendance }) {
  return (
    <section className="mt-5 grid gap-5 lg:grid-cols-2">
      <form className="card p-6 md:p-7" onSubmit={submitLeave}>
        <p className="eyebrow">Self Service</p><h2 className="mt-2 text-xl font-black text-emerald-950">Ajukan izin atau cuti</h2>
        <div className="mt-5 grid gap-3 sm:grid-cols-2">
          <input className="input" type="date" value={leaveForm.start_date} onChange={(e) => setLeaveForm({ ...leaveForm, start_date: e.target.value })} required />
          <input className="input" type="date" value={leaveForm.end_date} onChange={(e) => setLeaveForm({ ...leaveForm, end_date: e.target.value })} required />
          <select className="input sm:col-span-2" value={leaveForm.type} onChange={(e) => setLeaveForm({ ...leaveForm, type: e.target.value })}><option value="permission">Izin</option><option value="sick">Sakit</option><option value="leave">Cuti</option></select>
          <textarea className="input sm:col-span-2" placeholder="Tuliskan alasan singkat dan jelas" value={leaveForm.reason} onChange={(e) => setLeaveForm({ ...leaveForm, reason: e.target.value })} required />
        </div>
        <button className="btn mt-4 w-full" disabled={busy === "leave"}>Kirim Pengajuan</button>
      </form>

      <div className="card p-6 md:p-7">
        <p className="eyebrow">Riwayat Saya</p><h2 className="mt-2 text-xl font-black text-emerald-950">Kehadiran & pengajuan</h2>
        <div className="mt-5 space-y-3">
          {attendance.slice(0, 4).map((row) => <div className="ops-history" key={`att-${row.id}`}><div><p className="font-bold text-emerald-950">Kehadiran · {formatDate(row.work_date)}</p><p className="text-xs text-slate-500">{formatTime(row.clock_in)} – {formatTime(row.clock_out)}</p></div><Pill tone="green">{statusLabel(row.status)}</Pill></div>)}
          {timeOff.slice(0, 4).map((row) => <div className="ops-history" key={`off-${row.id}`}><div><p className="font-bold text-emerald-950">{timeOffTypeLabel(row.type)} · {formatDate(row.start_date)}</p><p className="text-xs text-slate-500">{row.reason}</p></div><Pill tone={row.status === "approved" ? "green" : row.status === "rejected" ? "red" : "amber"}>{statusLabel(row.status)}</Pill></div>)}
          {!attendance.length && !timeOff.length && <Empty>Belum ada riwayat kerja yang tercatat.</Empty>}
        </div>
      </div>
    </section>
  );
}
