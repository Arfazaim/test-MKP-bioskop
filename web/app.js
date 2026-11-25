(() => {
  const POSTERS = [
    "/static/posters/dori-the-explorer.jpg",
    "/static/posters/ari-n-the-gank.jpg",
    "/static/posters/badut-bopong.jpg",
  ];

  const state = {
    view: "katalog",
    token: localStorage.getItem("cine_token") || "",
    user: JSON.parse(localStorage.getItem("cine_user") || "null"),
    authMode: "login",
    afterLoginCallback: null,

    movies: [],
    cinemas: [],
    studios: [],
    cities: [],
    schedules: [],
    myBookings: [],

    selectedMovieId: null,
    searchQuery: "",
    filterCity: "Semua",
    filterFormat: "Semua",
    filterDate: "Semua",

    activeSchedule: null,
    scheduleSeats: [],
    selectedSeatIds: new Set(),

    activeBooking: null,
    promoCode: "",
    paymentMethod: "QRIS",
    countdownInterval: null,

    editingScheduleId: null,
  };

  function posterFor(movieId) {
    return POSTERS[(Number(movieId || 1) - 1) % POSTERS.length];
  }

  function escapeHtml(str) {
    return String(str ?? "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function formatRupiah(amount) {
    return "Rp " + Number(amount || 0).toLocaleString("id-ID");
  }

  function formatTimeWIB(iso) {
    if (!iso) return "-";
    const d = new Date(iso);
    return d.toLocaleTimeString("id-ID", { hour: "2-digit", minute: "2-digit", timeZone: "Asia/Jakarta" }).replace(".", ":") + " WIB";
  }

  function formatDateID(iso) {
    if (!iso) return "-";
    const d = new Date(iso);
    return d.toLocaleDateString("id-ID", { weekday: "short", day: "numeric", month: "short", year: "numeric", timeZone: "Asia/Jakarta" });
  }

  function isoDateWIB(iso) {
    const d = new Date(iso);
    return new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Jakarta", year: "numeric", month: "2-digit", day: "2-digit" }).format(d);
  }

  let toastTimer = null;
  function showToast(msg) {
    const el = document.getElementById("toast");
    const txt = document.getElementById("toast-text");
    txt.textContent = msg;
    el.classList.remove("opacity-0", "translate-y-4");
    el.classList.add("opacity-100", "translate-y-0");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
      el.classList.remove("opacity-100", "translate-y-0");
      el.classList.add("opacity-0", "translate-y-4");
    }, 3200);
  }

  async function api(path, options = {}) {
    const headers = Object.assign({ "Content-Type": "application/json" }, options.headers || {});
    if (state.token) {
      headers["Authorization"] = "Bearer " + state.token;
    }
    const resp = await fetch(path, Object.assign({}, options, { headers }));
    const json = await resp.json().catch(() => ({}));
    if (!resp.ok) {
      if (resp.status === 401 && state.token) {
        logout(true);
      }
      throw new Error(json.error || "Terjadi kesalahan pada permintaan");
    }
    return json;
  }

  async function loadInitialData() {
    try {
      const [metaRes, schedRes] = await Promise.all([
        api("/api/v1/catalog/metadata"),
        api("/api/v1/catalog/schedules?limit=100"),
      ]);
      state.movies = metaRes.data.movies || [];
      state.cinemas = metaRes.data.cinemas || [];
      state.studios = metaRes.data.studios || [];
      state.cities = metaRes.data.cities || [];
      state.schedules = (schedRes.data && schedRes.data.items) || [];

      if (!state.selectedMovieId && state.movies.length > 0) {
        state.selectedMovieId = state.movies[0].id;
      }

      if (state.token) {
        try {
          const me = await api("/api/v1/auth/me");
          state.user = me.data;
          localStorage.setItem("cine_user", JSON.stringify(state.user));
          await refreshMyBookings();
        } catch (_) {
          logout(true);
        }
      }
    } catch (err) {
      showToast(err.message);
    }
    renderChrome();
    renderView();
  }

  async function refreshSchedules() {
    const schedRes = await api("/api/v1/catalog/schedules?limit=100");
    state.schedules = (schedRes.data && schedRes.data.items) || [];
  }

  async function refreshMyBookings() {
    if (!state.token) {
      state.myBookings = [];
      return;
    }
    const res = await api("/api/v1/bookings");
    state.myBookings = res.data || [];
  }

  function renderChrome() {
    document.querySelectorAll(".nav-pill").forEach((btn) => {
      const target = btn.dataset.nav;
      const isActive =
        state.view === target ||
        (target === "bioskop" && (state.view === "pilih-kursi" || state.view === "checkout")) ||
        (target === "tiket-saya" && state.view === "etiket");
      if (target === "admin") {
        if (state.user && state.user.role === "ADMIN") {
          btn.classList.remove("hidden");
          btn.classList.add("flex");
          btn.className = isActive
            ? "nav-pill flex items-center space-x-2 bg-amber-400 text-black text-xs lg:text-sm font-extrabold uppercase tracking-wide px-4 py-2 rounded-none border-2 border-white shadow-[4px_4px_0px_#e50914] transition-all"
            : "nav-pill flex items-center space-x-2 bg-amber-400/90 hover:bg-amber-300 text-black text-xs lg:text-sm font-extrabold uppercase tracking-wide px-4 py-2 rounded-none border-2 border-white shadow-[4px_4px_0px_#000000] transition-all hover:-translate-x-0.5 hover:-translate-y-0.5";
        } else {
          btn.classList.add("hidden");
          btn.classList.remove("flex");
        }
        return;
      }
      if (isActive) {
        btn.className = "nav-pill flex items-center space-x-2 bg-[#e50914] text-white text-xs lg:text-sm font-extrabold uppercase tracking-wide px-4 py-2 rounded-none border-2 border-white shadow-[4px_4px_0px_#000000] transition-transform hover:-translate-x-0.5 hover:-translate-y-0.5";
      } else {
        btn.className = "nav-pill flex items-center space-x-2 bg-[#1f1f23] hover:bg-[#2a292e] text-slate-200 text-xs lg:text-sm font-bold uppercase tracking-wide px-4 py-2 rounded-none border-2 border-white/70 shadow-[4px_4px_0px_#000000] transition-all hover:-translate-x-0.5 hover:-translate-y-0.5";
      }
    });

    const badge = document.getElementById("my-tickets-badge");
    if (state.myBookings.length > 0) {
      badge.textContent = state.myBookings.length;
      badge.classList.remove("hidden");
    } else {
      badge.classList.add("hidden");
    }

    const authBar = document.getElementById("auth-bar");
    if (state.user) {
      const roleClass = state.user.role === "ADMIN"
        ? "bg-amber-400 text-black border-black"
        : "bg-[#e50914] text-white border-white";
      const roleLabel = state.user.role === "ADMIN" ? "Admin" : "Pelanggan";
      authBar.innerHTML = `
        <div class="flex items-center gap-2.5 bg-[#1f1f23] border-2 border-white/80 rounded-none pl-3 pr-1.5 py-1 shadow-[3px_3px_0px_#000000]">
          <div class="text-right">
            <div class="text-xs font-extrabold text-white leading-tight truncate max-w-[120px]">${escapeHtml(state.user.full_name)}</div>
            <div class="text-[9.5px] font-extrabold uppercase tracking-wider px-1.5 py-0.5 rounded-none border inline-block mt-0.5 leading-none ${roleClass}">${roleLabel}</div>
          </div>
          <button onclick="window.app.logout()" title="Keluar" class="w-8 h-8 rounded-none bg-[#131317] hover:bg-[#e50914] text-slate-200 hover:text-white border-2 border-white/70 flex items-center justify-center transition-colors">
            <i class="fa-solid fa-right-from-bracket text-xs"></i>
          </button>
        </div>
      `;
    } else {
      authBar.innerHTML = `
        <button onclick="window.app.openAuthModal('login')" class="flex items-center gap-2 bg-[#e50914] hover:bg-[#c0000c] text-white text-xs font-extrabold uppercase tracking-wide px-4 py-2.5 rounded-none border-2 border-white shadow-[4px_4px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all">
          <i class="fa-regular fa-user text-xs"></i>
          <span>Masuk / Daftar</span>
        </button>
      `;
    }

    const stripBar = document.getElementById("thumbnails-bar");
    if (state.view === "katalog" || state.view === "bioskop") {
      stripBar.classList.remove("hidden");
    } else {
      stripBar.classList.add("hidden");
    }

    document.getElementById("movie-count-badge").textContent = `${state.movies.length} Film`;
    const strip = document.getElementById("movie-thumbnails");
    strip.innerHTML = state.movies
      .map((m) => {
        const active = m.id === state.selectedMovieId;
        return `
          <div onclick="window.app.selectMovie(${m.id})" class="relative flex-shrink-0 cursor-pointer group">
            <div class="w-20 h-28 lg:w-24 lg:h-32 rounded-none overflow-hidden ${
              active
                ? "border-[3px] border-[#e50914] shadow-[4px_4px_0px_#e50914]"
                : "border-2 border-white/60 shadow-[3px_3px_0px_#000000] opacity-80 hover:opacity-100 hover:border-white"
            } transition-all group-hover:-translate-x-0.5 group-hover:-translate-y-0.5 bg-[#1f1f23]">
              <img alt="${escapeHtml(m.title)}" src="${posterFor(m.id)}" class="w-full h-full object-cover"/>
              <div class="absolute inset-0 bg-gradient-to-t from-black/85 via-black/15 to-transparent"></div>
            </div>
            <span class="absolute bottom-1.5 left-1/2 -translate-x-1/2 text-[9px] ${
              active ? "font-extrabold text-white" : "font-bold text-slate-200 bg-black/70"
            } uppercase tracking-wider text-center w-full px-1 truncate">${escapeHtml(m.title)}</span>
          </div>
        `;
      })
      .join("");
  }

  function stopCountdown() {
    if (state.countdownInterval) {
      clearInterval(state.countdownInterval);
      state.countdownInterval = null;
    }
  }

  function navigate(view) {
    stopCountdown();
    state.view = view;
    renderChrome();
    renderView();
  }

  function renderView() {
    const root = document.getElementById("app-view");
    if (state.view === "katalog") return renderKatalog(root);
    if (state.view === "bioskop") return renderBioskop(root);
    if (state.view === "pilih-kursi") return renderSeatSelection(root);
    if (state.view === "checkout") return renderCheckout(root);
    if (state.view === "etiket") return renderETicket(root);
    if (state.view === "tiket-saya") return renderMyTickets(root);
    if (state.view === "admin") return renderAdminPanel(root);
  }

  // ==================== VIEW 1: KATALOG FILM (NEO-BRUTALISM HERO) ====================
  function renderKatalog(root) {
    const movie = state.movies.find((m) => m.id === state.selectedMovieId) || state.movies[0];
    if (!movie) {
      root.innerHTML = `<div class="bg-[#1f1f23] border-2 border-white p-10 text-center text-sm font-bold text-slate-300 shadow-[6px_6px_0px_#000000]">Belum ada film yang sedang tayang saat ini.</div>`;
      return;
    }

    const movieSchedules = state.schedules.filter(
      (s) => s.movie_id === movie.id && s.status === "SCHEDULED"
    );
    const minPrice = movieSchedules.length > 0
      ? Math.min(...movieSchedules.map((s) => s.price))
      : 50000;

    const quickBtns = movieSchedules.slice(0, 4).map((s) => `
      <button onclick="window.app.openSeatMap(${s.id})" class="py-2 px-3.5 text-xs font-extrabold uppercase tracking-wide rounded-none bg-[#1f1f23] hover:bg-[#e50914] border-2 border-white text-white shadow-[3px_3px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all text-left">
        <span>${escapeHtml(s.studio?.cinema?.name || "Bioskop")}</span>
        <span class="text-[#e50914] group-hover:text-white mx-1">•</span>
        <span class="tabular-nums">${formatTimeWIB(s.start_time)}</span>
      </button>
    `).join("");

    root.innerHTML = `
      <div class="relative py-6 px-5 sm:px-8 min-h-[450px] flex items-center overflow-hidden bg-[#131317] border-[3px] border-white/85 shadow-[6px_6px_0px_#000000]">
        <div class="absolute right-0 top-0 bottom-0 w-full sm:w-3/4 lg:w-3/5 pointer-events-none select-none overflow-hidden">
          <img alt="${escapeHtml(movie.title)}" src="${posterFor(movie.id)}" class="w-full h-full object-cover object-center scale-105 opacity-75 filter contrast-125 saturate-110"/>
          <div class="absolute inset-0 bg-gradient-to-r from-[#131317] via-[#131317]/80 to-transparent"></div>
          <div class="absolute inset-0 bg-gradient-to-t from-[#131317] via-transparent to-transparent"></div>
        </div>

        <div class="relative z-10 max-w-2xl py-2 space-y-4">
          <div class="flex flex-wrap items-center gap-2.5">
            <div class="flex items-center space-x-1.5 bg-[#e50914] text-white text-[11px] font-extrabold uppercase px-2.5 py-1 rounded-none border-2 border-black shadow-[3px_3px_0px_#000000]">
              <i class="fa-solid fa-fire text-[10px]"></i>
              <span>Promo DISKON25</span>
            </div>
            <div class="flex items-center space-x-1.5 bg-[#1f1f23] border-2 border-white/60 text-white text-[11px] font-bold uppercase px-2.5 py-1 rounded-none shadow-[3px_3px_0px_#000000]">
              <span class="w-2 h-2 rounded-none bg-emerald-400 border border-black"></span>
              <span>Sedang Tayang</span>
            </div>
            <span class="bg-black border-2 border-white/60 text-white text-[10px] font-extrabold uppercase px-2 py-0.5 rounded-none shadow-[2px_2px_0px_#000000]">${escapeHtml(movie.genre || "Umum")}</span>
            <span class="bg-black border-2 border-amber-400 text-amber-400 text-[10px] font-extrabold uppercase px-2 py-0.5 rounded-none shadow-[2px_2px_0px_#000000]">${escapeHtml(movie.rating || "SU")}</span>
            <span class="text-xs font-extrabold text-white flex items-center gap-1 ml-1">
              <i class="fa-regular fa-clock text-[#e50914] text-xs"></i> ${movie.duration_minutes} Menit
            </span>
          </div>

          <div>
            <h1 class="text-3xl sm:text-4xl lg:text-5xl font-black tracking-tight text-white leading-tight uppercase">
              ${escapeHtml(movie.title)}
            </h1>
            <p class="text-xs sm:text-sm text-slate-200 mt-2.5 max-w-xl font-medium leading-relaxed">
              ${escapeHtml(movie.synopsis || "Nikmati penayangan layar lebar dengan kualitas gambar dan suara terbaik di jaringan bioskop Zanema.")}
            </p>
          </div>

          <div class="flex items-baseline space-x-3 pt-1">
            <span class="text-xs font-bold uppercase tracking-wider text-slate-300">Mulai dari</span>
            <span class="text-2xl sm:text-3xl font-black text-white tracking-tight tabular-nums">${formatRupiah(minPrice)}</span>
          </div>

          <div class="flex items-center flex-wrap gap-3.5 pt-2">
            <button onclick="window.app.buyForMovie(${movie.id})" class="flex items-center space-x-2.5 bg-[#e50914] hover:bg-[#c0000c] text-white text-sm font-extrabold uppercase tracking-wide px-6 py-3 rounded-none border-2 border-white shadow-[5px_5px_0px_#000000] transition-all hover:-translate-x-0.5 hover:-translate-y-0.5 active:translate-x-[2px] active:translate-y-[2px] active:shadow-none">
              <i class="fa-solid fa-ticket text-sm"></i>
              <span>Beli Tiket Sekarang</span>
            </button>
            <button onclick="window.app.navigate('bioskop')" class="flex items-center space-x-2 bg-[#1f1f23] hover:bg-[#2a292e] text-white text-sm font-bold uppercase tracking-wide px-5 py-3 rounded-none border-2 border-white/70 shadow-[4px_4px_0px_#000000] transition-all hover:-translate-x-0.5 hover:-translate-y-0.5 active:translate-x-[2px] active:translate-y-[2px] active:shadow-none">
              <i class="fa-solid fa-calendar-days text-xs text-slate-300"></i>
              <span>Lihat Semua Jadwal</span>
            </button>
          </div>

          <div class="mt-4 pt-4 border-t-2 border-white/30 max-w-xl">
            <p class="text-[11px] font-extrabold text-slate-200 uppercase tracking-wider mb-3 flex items-center gap-1.5">
              <i class="fa-regular fa-clock text-[#e50914]"></i> Pilih Cepat Jam Tayang Tersedia
            </p>
            <div class="flex flex-wrap gap-2.5">
              ${quickBtns || `<span class="text-xs font-semibold text-slate-400">Belum ada jadwal aktif untuk film ini.</span>`}
            </div>
          </div>
        </div>
      </div>
    `;
  }

  // ==================== VIEW 2: JADWAL & BIOSKOP ====================
  function renderBioskop(root) {
    const activeSchedules = state.schedules.filter((s) => s.status !== "CANCELLED");
    const uniqueDates = Array.from(new Set(activeSchedules.map((s) => isoDateWIB(s.start_time)))).sort();

    const filtered = activeSchedules.filter((s) => {
      const cinema = s.studio?.cinema;
      const cityName = cinema?.city?.name || "";
      const studioType = s.studio?.studio_type || "";
      const movieTitle = s.movie?.title || "";
      const q = state.searchQuery.trim().toLowerCase();

      if (state.filterCity !== "Semua" && cityName !== state.filterCity) return false;
      if (state.filterFormat !== "Semua" && studioType !== state.filterFormat) return false;
      if (state.filterDate !== "Semua" && isoDateWIB(s.start_time) !== state.filterDate) return false;
      if (q && !movieTitle.toLowerCase().includes(q) && !(cinema?.name || "").toLowerCase().includes(q)) return false;
      return true;
    });

    const byCinema = new Map();
    filtered.forEach((s) => {
      const c = s.studio?.cinema;
      if (!c) return;
      if (!byCinema.has(c.id)) {
        byCinema.set(c.id, { cinema: c, byMovie: new Map() });
      }
      const group = byCinema.get(c.id);
      const m = s.movie;
      if (!group.byMovie.has(m.id)) {
        group.byMovie.set(m.id, { movie: m, schedules: [] });
      }
      group.byMovie.get(m.id).schedules.push(s);
    });

    const cityOptions = [`<option value="Semua">Semua Kota</option>`]
      .concat(
        state.cities.map(
          (c) => `<option value="${escapeHtml(c.name)}" ${state.filterCity === c.name ? "selected" : ""}>${escapeHtml(c.name)}</option>`
        )
      )
      .join("");

    const formats = ["Semua", "REGULAR", "IMAX"];
    const formatChips = formats
      .map((f) => {
        const active = state.filterFormat === f;
        return `<button onclick="window.app.setFilter('format','${f}')" class="px-4 py-2 rounded-none border-2 font-extrabold text-xs uppercase tracking-wide shadow-[4px_4px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all whitespace-nowrap ${
          active ? "bg-[#e50914] text-white border-white" : "bg-[#131317] text-slate-200 border-white/80 hover:bg-[#2a292e] hover:text-white"
        }">${f}</button>`;
      })
      .join("");

    const dateChips = [`Semua`, ...uniqueDates]
      .map((d) => {
        const active = state.filterDate === d;
        const label = d === "Semua" ? "Semua Tanggal" : formatDateID(d + "T00:00:00+07:00");
        return `<button onclick="window.app.setFilter('date','${d}')" class="px-5 py-2.5 rounded-none border-2 font-extrabold text-xs uppercase tracking-wide shadow-[4px_4px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all whitespace-nowrap ${
          active ? "bg-[#e50914] text-white border-white" : "bg-[#131317] text-slate-200 border-white/80 hover:bg-[#2a292e] hover:text-white"
        }">${escapeHtml(label)}</button>`;
      })
      .join("");

    let cinemaCardsHtml = "";
    byCinema.forEach(({ cinema, byMovie }) => {
      let moviesHtml = "";
      byMovie.forEach(({ movie, schedules }) => {
        const btns = schedules
          .map(
            (s) => `
            <button onclick="window.app.openSeatMap(${s.id})" class="px-4 py-2.5 rounded-none border-2 border-white/80 bg-[#1f1f23] hover:bg-[#e50914] hover:border-white text-white text-xs font-extrabold shadow-[4px_4px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all flex flex-col items-center min-w-[110px]">
              <span class="tabular-nums text-sm">${formatTimeWIB(s.start_time)}</span>
              <span class="text-[10px] font-bold opacity-85 mt-0.5">${escapeHtml(s.studio?.name || "")} (${escapeHtml(s.studio?.studio_type || "")}) • ${formatRupiah(s.price)}</span>
            </button>
          `
          )
          .join("");

        moviesHtml += `
          <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-5 p-4 rounded-none border-2 border-white/40 bg-[#131317]">
            <div class="flex items-start gap-4 min-w-0">
              <img src="${posterFor(movie.id)}" alt="${escapeHtml(movie.title)}" class="w-16 h-24 rounded-none border-2 border-white/80 object-cover shrink-0 shadow-[3px_3px_0px_#000000]"/>
              <div class="flex flex-col gap-1.5 min-w-0">
                <div class="flex items-center gap-2 flex-wrap">
                  <h4 class="text-lg font-extrabold text-white uppercase tracking-tight">${escapeHtml(movie.title)}</h4>
                  <span class="px-2 py-0.5 rounded-none border border-white/80 bg-[#1f1f23] text-white text-[11px] font-bold">${escapeHtml(movie.rating || "SU")}</span>
                </div>
                <div class="flex flex-wrap items-center gap-2 text-xs text-slate-300 font-medium">
                  <span class="text-[#e50914] font-bold">${escapeHtml(movie.genre || "Film")}</span>
                  <span>•</span>
                  <span>${movie.duration_minutes} Menit</span>
                  <span>•</span>
                  <span class="text-white font-bold">${formatDateID(schedules[0].start_time)}</span>
                </div>
              </div>
            </div>
            <div class="flex flex-wrap items-center gap-2.5 shrink-0">${btns}</div>
          </div>
        `;
      });

      cinemaCardsHtml += `
        <article class="bg-[#1f1f23] rounded-none border-2 border-white/90 p-6 shadow-[6px_6px_0px_#e50914] flex flex-col gap-5">
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-4 bg-[#131317] border-b-2 border-white/80 -mx-6 -mt-6 px-6 pt-5">
            <div class="flex items-start gap-4">
              <div class="w-12 h-12 rounded-none border-2 border-[#e50914] bg-[#e50914]/20 flex items-center justify-center text-[#e50914] shrink-0 shadow-[3px_3px_0px_#000000]">
                <span class="material-symbols-outlined text-[26px]">theaters</span>
              </div>
              <div>
                <h3 class="text-xl font-extrabold text-white uppercase tracking-tight">${escapeHtml(cinema.name)}</h3>
                <p class="text-xs text-slate-300 mt-0.5">${escapeHtml(cinema.address)} • ${escapeHtml(cinema.city?.name || "")}</p>
              </div>
            </div>
            <span class="px-3 py-1.5 rounded-none border-2 border-white/80 bg-[#1f1f23] text-xs font-extrabold uppercase text-white shadow-[3px_3px_0px_#000000] self-start sm:self-center">
              ${escapeHtml(cinema.city?.name || "Zanema")}
            </span>
          </div>
          ${moviesHtml}
        </article>
      `;
    });

    root.innerHTML = `
      <div class="flex flex-col w-full gap-7">
        <section class="w-full bg-[#1f1f23] rounded-none border-2 border-white/90 p-5 sm:p-6 shadow-[6px_6px_0px_#000000] flex flex-col gap-5">
          <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
            <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 flex-1">
              <div class="relative min-w-[200px]">
                <span class="material-symbols-outlined absolute left-3.5 top-1/2 -translate-y-1/2 text-[#e50914] text-[20px] z-10">location_on</span>
                <select onchange="window.app.setFilter('city', this.value)" class="w-full bg-[#131317] text-white text-xs font-bold pl-10 pr-9 py-3 rounded-none border-2 border-white/90 shadow-[4px_4px_0px_#000000] focus:outline-none focus:border-[#e50914] focus:ring-0">
                  ${cityOptions}
                </select>
              </div>
              <div class="relative flex-1 max-w-md">
                <span class="material-symbols-outlined absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-300 text-[20px]">search</span>
                <input value="${escapeHtml(state.searchQuery)}" oninput="window.app.setFilter('search', this.value)" type="text" placeholder="Cari bioskop atau judul film..." class="w-full bg-[#131317] text-white placeholder-slate-400 text-xs font-medium pl-10 pr-4 py-3 rounded-none border-2 border-white/90 shadow-[4px_4px_0px_#000000] focus:outline-none focus:border-[#e50914] focus:ring-0"/>
              </div>
            </div>
            <div class="flex items-center gap-2.5 overflow-x-auto no-scrollbar py-1 pr-1">${formatChips}</div>
          </div>
          <div class="flex items-center gap-2.5 overflow-x-auto no-scrollbar py-1 pr-1">${dateChips}</div>
        </section>

        <section class="flex flex-col gap-6">
          ${cinemaCardsHtml || `<div class="bg-[#1f1f23] rounded-none border-2 border-white/80 p-10 text-center text-sm font-bold text-slate-300 shadow-[6px_6px_0px_#000000]">Tidak ada jadwal tayang yang cocok dengan filter saat ini.</div>`}
        </section>
      </div>
    `;
  }

  // ==================== VIEW 3: PEMILIHAN KURSI ====================
  async function openSeatMap(scheduleId) {
    try {
      const res = await api(`/api/v1/catalog/schedules/${scheduleId}/seats`);
      state.activeSchedule = res.data.schedule;
      state.scheduleSeats = res.data.seats || [];
      state.selectedSeatIds = new Set();
      navigate("pilih-kursi");
    } catch (err) {
      showToast(err.message);
    }
  }

  function renderSeatSelection(root) {
    const s = state.activeSchedule;
    if (!s) return navigate("bioskop");

    const rowsMap = new Map();
    state.scheduleSeats.forEach((seat) => {
      if (!rowsMap.has(seat.row_label)) rowsMap.set(seat.row_label, []);
      rowsMap.get(seat.row_label).push(seat);
    });

    const selectedSeats = state.scheduleSeats.filter((st) => state.selectedSeatIds.has(st.id));
    const selectedLabels = selectedSeats.map((st) => `${st.row_label}${st.seat_number}`);
    const count = selectedSeats.length;
    const fee = count > 0 ? 4000 : 0;
    const total = count > 0 ? count * s.price + fee : 0;

    let gridHtml = "";
    rowsMap.forEach((seatsInRow, rowLabel) => {
      const buttons = seatsInRow
        .map((seat, idx) => {
          const aisle = idx === 5 ? `<div class="w-5"></div>` : "";
          const isPicked = state.selectedSeatIds.has(seat.id);
          let cls = "";
          let content = seat.seat_number;
          let disabled = "";
          let title = `Kursi ${seat.row_label}${seat.seat_number}`;

          if (seat.status === "BOOKED") {
            disabled = "disabled";
            cls = "w-8 h-8 rounded-none bg-[#0e0e12] border-2 border-white/25 text-white/30 flex items-center justify-center cursor-not-allowed";
            content = `<span class="material-symbols-outlined text-[14px]">close</span>`;
            title += " (Sudah Terjual)";
          } else if (seat.status === "LOCKED") {
            disabled = "disabled";
            cls = "w-8 h-8 rounded-none bg-amber-400/20 border-2 border-amber-400 text-amber-300 flex items-center justify-center cursor-not-allowed";
            content = `<i class="fa-solid fa-lock text-[10px]"></i>`;
            title += " (Sedang Ditahan Pembeli Lain)";
          } else if (isPicked) {
            cls = "w-8 h-8 rounded-none bg-[#e50914] border-2 border-white text-white text-xs font-extrabold flex items-center justify-center shadow-[3px_3px_0px_#000000] -translate-x-0.5 -translate-y-0.5 transition-all";
          } else {
            cls = "w-8 h-8 rounded-none bg-[#2a292e] hover:bg-[#39393d] border-2 border-white/60 hover:border-white text-slate-200 hover:text-white text-xs font-bold flex items-center justify-center shadow-[2px_2px_0px_#000000] transition-all";
          }

          return `${aisle}<button type="button" ${disabled} title="${title}" onclick="window.app.toggleSeat(${seat.id})" class="${cls}">${content}</button>`;
        })
        .join("");

      gridHtml += `
        <div class="flex items-center gap-2">
          <span class="w-5 text-center text-xs font-extrabold text-slate-400 select-none">${escapeHtml(rowLabel)}</span>
          ${buttons}
          <span class="w-5 text-center text-xs font-extrabold text-slate-400 select-none">${escapeHtml(rowLabel)}</span>
        </div>
      `;
    });

    root.innerHTML = `
      <div class="flex flex-col w-full">
        <div class="w-full bg-[#1f1f23] rounded-none border-[3px] border-white/85 p-5 mb-6 flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4 shadow-[6px_6px_0px_#000000]">
          <div class="flex items-center gap-4 min-w-0">
            <img src="${posterFor(s.movie_id)}" class="w-16 h-20 rounded-none border-2 border-white/80 object-cover shrink-0 shadow-[3px_3px_0px_#000000]"/>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2.5 mb-1.5">
                <h1 class="text-xl font-extrabold text-white uppercase tracking-tight truncate">${escapeHtml(s.movie?.title || "")}</h1>
                <span class="px-2.5 py-0.5 rounded-none bg-[#e50914] text-white text-xs font-extrabold uppercase border-2 border-white shadow-[3px_3px_0px_#000000]">${escapeHtml(s.studio?.studio_type || "REGULAR")}</span>
              </div>
              <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-slate-300 font-medium">
                <span class="flex items-center gap-1"><span class="material-symbols-outlined text-[15px] text-[#e50914]">location_on</span>${escapeHtml(s.studio?.cinema?.name || "")} (${escapeHtml(s.studio?.name || "")})</span>
                <span>•</span>
                <span>${formatDateID(s.start_time)}</span>
                <span>•</span>
                <span class="text-white font-extrabold">${formatTimeWIB(s.start_time)}</span>
              </div>
            </div>
          </div>
          <button onclick="window.app.navigate('bioskop')" class="px-5 py-2.5 rounded-none bg-[#2a292e] hover:bg-[#39393d] text-white text-xs font-extrabold uppercase tracking-wide border-2 border-white shadow-[4px_4px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all flex items-center gap-2 self-start lg:self-center">
            <span class="material-symbols-outlined text-[18px]">edit_calendar</span>
            <span>Ubah Jadwal</span>
          </button>
        </div>

        <div class="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
          <div class="lg:col-span-7 bg-[#1f1f23] rounded-none border-[3px] border-white/85 p-6 lg:p-8 flex flex-col items-center shadow-[6px_6px_0px_#000000]">
            <div class="w-full max-w-lg flex flex-col items-center mb-9">
              <div class="w-full h-3.5 rounded-none bg-[#e50914] border-2 border-white shadow-[4px_4px_0px_#000000]"></div>
              <span class="mt-3 text-xs font-extrabold uppercase tracking-[0.25em] text-slate-200">LAYAR BIOSKOP (${escapeHtml(s.studio?.studio_type || "STUDIO")})</span>
            </div>

            <div class="w-full overflow-x-auto pb-3">
              <div class="min-w-[440px] flex flex-col gap-2.5 items-center mx-auto">${gridHtml}</div>
            </div>

            <div class="w-full mt-7 bg-[#131317] rounded-none border-2 border-white/70 px-5 py-3.5 flex flex-wrap items-center justify-center gap-5 text-xs shadow-[4px_4px_0px_#000000]">
              <div class="flex items-center gap-2"><div class="w-4 h-4 rounded-none bg-[#2a292e] border-2 border-white/70"></div><span class="font-bold text-slate-300">Tersedia</span></div>
              <div class="flex items-center gap-2"><div class="w-4 h-4 rounded-none bg-[#e50914] border-2 border-white shadow-[2px_2px_0px_#000000]"></div><span class="font-bold text-white">Dipilih</span></div>
              <div class="flex items-center gap-2"><div class="w-4 h-4 rounded-none bg-amber-400/20 border-2 border-amber-400 flex items-center justify-center text-amber-300"><i class="fa-solid fa-lock text-[8px]"></i></div><span class="font-bold text-amber-300">Ditahan 10 Menit</span></div>
              <div class="flex items-center gap-2"><div class="w-4 h-4 rounded-none bg-[#0e0e12] border-2 border-white/30 flex items-center justify-center text-white/40"><span class="material-symbols-outlined text-[11px]">close</span></div><span class="font-bold text-slate-400">Terjual</span></div>
            </div>
          </div>

          <div class="lg:col-span-5 bg-[#1f1f23] rounded-none border-[3px] border-white p-6 lg:p-8 flex flex-col shadow-[6px_6px_0px_#e50914]">
            <div class="flex items-center justify-between mb-5">
              <h2 class="text-lg font-extrabold text-white uppercase tracking-tight">Ringkasan Pesanan</h2>
              <span class="text-[11px] font-extrabold text-white uppercase bg-[#131317] border-2 border-white px-2.5 py-1 rounded-none">${escapeHtml(s.studio?.name || "")}</span>
            </div>

            <div class="flex flex-col gap-3.5 mb-6 text-sm bg-[#131317] border-2 border-white/80 p-4 rounded-none shadow-[4px_4px_0px_#000000]">
              <div class="flex items-center justify-between"><span class="text-slate-300">Kursi Dipilih</span><span class="font-extrabold text-[#e50914]">${selectedLabels.length ? selectedLabels.join(", ") : "-"}</span></div>
              <div class="flex items-center justify-between"><span class="text-slate-300">Jumlah Tiket</span><span class="font-extrabold text-white">${count} Tiket</span></div>
              <div class="flex items-center justify-between"><span class="text-slate-300">Harga per Tiket</span><span class="tabular-nums font-bold text-white">${formatRupiah(s.price)}</span></div>
              <div class="flex items-center justify-between"><span class="text-slate-300">Biaya Layanan</span><span class="tabular-nums font-bold text-white">${formatRupiah(fee)}</span></div>
            </div>

            <div class="bg-[#131317] border-2 border-white rounded-none p-4 mb-6 flex items-center justify-between shadow-[4px_4px_0px_#000000]">
              <div>
                <div class="text-[11px] font-extrabold text-slate-300 uppercase tracking-wider">Total Sementara</div>
                <div class="text-[11px] text-slate-400">Kursi ditahan aman 10 menit saat lanjut</div>
              </div>
              <span class="text-2xl font-black text-[#e50914] tabular-nums">${formatRupiah(total)}</span>
            </div>

            <button onclick="window.app.lockSeatsAndCheckout()" ${count === 0 ? "disabled" : ""} class="w-full py-3.5 px-6 rounded-none border-[3px] ${
              count === 0
                ? "bg-[#2a292e] border-white/30 text-slate-500 cursor-not-allowed"
                : "bg-[#e50914] hover:bg-[#c0000c] border-white text-white shadow-[5px_5px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none"
            } text-sm font-extrabold uppercase tracking-wider transition-all flex items-center justify-center gap-2">
              <span>Lanjut ke Pembayaran</span>
              <span class="material-symbols-outlined text-[18px]">arrow_forward</span>
            </button>
          </div>
        </div>
      </div>
    `;
  }

  async function lockSeatsAndCheckout() {
    if (state.selectedSeatIds.size === 0) return;
    if (!state.token) {
      state.afterLoginCallback = () => lockSeatsAndCheckout();
      openAuthModal("login");
      showToast("Silakan masuk terlebih dahulu untuk mengunci kursi");
      return;
    }

    try {
      const res = await api("/api/v1/bookings", {
        method: "POST",
        body: JSON.stringify({
          schedule_id: state.activeSchedule.id,
          seat_ids: Array.from(state.selectedSeatIds),
        }),
      });
      state.activeBooking = res.data;
      state.promoCode = "";
      await refreshMyBookings();
      showToast(res.message);
      navigate("checkout");
    } catch (err) {
      showToast(err.message);
      if (state.activeSchedule) {
        await openSeatMap(state.activeSchedule.id);
      }
    }
  }

  // ==================== VIEW 4: CHECKOUT & PEMBAYARAN ====================
  function renderCheckout(root) {
    const b = state.activeBooking;
    if (!b) return navigate("bioskop");

    const s = b.schedule || state.activeSchedule || {};
    const seatLabels = (b.seats || []).map((bs) => `${bs.seat?.row_label || ""}${bs.seat?.seat_number || ""}`).join(", ");
    const count = (b.seats || []).length;
    const baseTotal = Number(b.total_amount || 0);
    const fee = 4000;
    const discount = state.promoCode.toUpperCase() === "DISKON25" ? 25000 : 0;
    const finalTotal = Math.max(0, baseTotal + fee - discount);

    const methods = [
      { id: "QRIS", title: "QRIS", sub: "Scan lewat GoPay, OVO, Dana, ShopeePay, atau m-Banking", icon: "qr_code_scanner", badge: "Rekomendasi • Instan" },
      { id: "EWALLET", title: "GoPay / OVO / Dana", sub: "Hubungkan langsung ke aplikasi dompet digital", icon: "account_balance_wallet", badge: "" },
      { id: "VA", title: "Virtual Account (BCA, Mandiri, BNI)", sub: "Transfer otomatis tanpa unggah bukti bayar", icon: "account_balance", badge: "" },
      { id: "CARD", title: "Kartu Kredit / Debit", sub: "Visa, Mastercard, dan JCB dengan proteksi 3D Secure", icon: "credit_card", badge: "" },
    ];

    root.innerHTML = `
      <div class="w-full max-w-[1200px] mx-auto">
        <div class="flex flex-col md:flex-row md:items-end justify-between gap-4 mb-7">
          <div>
            <div class="flex items-center gap-2 mb-1">
              <span class="text-[11px] font-extrabold uppercase tracking-widest text-[#e50914]">Langkah 03 / 03</span>
              <span class="text-white/40">•</span>
              <span class="text-[11px] font-bold uppercase tracking-widest text-slate-300">Kode ${escapeHtml(b.booking_code)}</span>
            </div>
            <h1 class="text-2xl sm:text-3xl font-black text-white uppercase tracking-tight">Checkout &amp; Pembayaran</h1>
          </div>
          <div class="flex items-center gap-2 bg-[#1f1f23] px-4 py-2.5 rounded-none border-2 border-white shadow-[4px_4px_0px_#e50914]">
            <span class="material-symbols-outlined text-[#e50914] text-lg">timer</span>
            <span class="text-xs font-bold text-slate-200">Selesaikan dalam</span>
            <span id="checkout-timer" class="text-sm font-black text-[#e50914] tabular-nums">--:--</span>
          </div>
        </div>

        <div class="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
          <div class="lg:col-span-7 flex flex-col gap-6">
            <section class="bg-[#1f1f23] rounded-none border-[3px] border-white p-6 shadow-[6px_6px_0px_#000000]">
              <div class="flex items-center justify-between mb-5">
                <div class="flex items-center gap-3">
                  <span class="w-8 h-8 rounded-none bg-[#e50914] text-white border-2 border-white flex items-center justify-center text-xs font-extrabold shadow-[2px_2px_0px_#000000]">01</span>
                  <h2 class="text-lg font-extrabold text-white uppercase tracking-tight">Informasi Pemesan</h2>
                </div>
                <span class="text-xs font-semibold text-slate-300">E-Tiket dikirim ke akun</span>
              </div>
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3.5 text-xs">
                <div class="bg-[#131317] p-3.5 rounded-none border-2 border-white/80 shadow-[3px_3px_0px_#000000]">
                  <div class="text-slate-400 font-bold uppercase tracking-wider mb-0.5">Nama Lengkap</div>
                  <div class="font-extrabold text-white text-sm">${escapeHtml(state.user?.full_name || "-")}</div>
                </div>
                <div class="bg-[#131317] p-3.5 rounded-none border-2 border-white/80 shadow-[3px_3px_0px_#000000]">
                  <div class="text-slate-400 font-bold uppercase tracking-wider mb-0.5">Email Pengiriman Tiket</div>
                  <div class="font-extrabold text-white text-sm">${escapeHtml(state.user?.email || "-")}</div>
                </div>
              </div>
            </section>

            <section class="bg-[#1f1f23] rounded-none border-[3px] border-white p-6 shadow-[6px_6px_0px_#000000]">
              <div class="flex items-center justify-between gap-2 mb-4">
                <div class="flex items-center gap-3">
                  <span class="w-8 h-8 rounded-none bg-[#e50914] text-white border-2 border-white flex items-center justify-center text-xs font-extrabold shadow-[2px_2px_0px_#000000]">02</span>
                  <h2 class="text-lg font-extrabold text-white uppercase tracking-tight">Kode Promo / Voucher</h2>
                </div>
                <button type="button" onclick="window.app.applyPromo('DISKON25')" class="inline-flex items-center gap-1.5 bg-[#e50914] hover:bg-[#c0000c] text-white px-3 py-1 rounded-none border-2 border-white text-xs font-extrabold uppercase tracking-wider shadow-[3px_3px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all">
                  <span class="material-symbols-outlined text-sm">local_offer</span>
                  <span>DISKON25</span>
                </button>
              </div>
              <div class="flex flex-col sm:flex-row gap-3">
                <input id="promo-field" value="${escapeHtml(state.promoCode)}" placeholder="Masukkan kode promo (mis. DISKON25)" class="flex-1 bg-[#131317] text-sm font-medium text-white placeholder-slate-500 px-4 py-3 rounded-none border-2 border-white/80 uppercase shadow-[4px_4px_0px_#000000] focus:outline-none focus:border-[#e50914] focus:ring-0"/>
                <button type="button" onclick="window.app.applyPromo(document.getElementById('promo-field').value)" class="px-6 py-3 rounded-none bg-[#2a292e] hover:bg-[#e50914] border-2 border-white text-xs font-extrabold uppercase tracking-wider text-white shadow-[4px_4px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all">Terapkan</button>
              </div>
            </section>

            <section class="bg-[#1f1f23] rounded-none border-[3px] border-white p-6 shadow-[6px_6px_0px_#000000]">
              <div class="flex items-center justify-between mb-5">
                <div class="flex items-center gap-3">
                  <span class="w-8 h-8 rounded-none bg-[#e50914] text-white border-2 border-white flex items-center justify-center text-xs font-extrabold shadow-[2px_2px_0px_#000000]">03</span>
                  <h2 class="text-lg font-extrabold text-white uppercase tracking-tight">Pilih Metode Pembayaran</h2>
                </div>
                <span class="text-xs font-semibold text-slate-300">Verifikasi otomatis</span>
              </div>
              <div class="flex flex-col gap-3">
                ${methods
                  .map((m) => {
                    const active = state.paymentMethod === m.id;
                    return `
                      <div onclick="window.app.setPaymentMethod('${m.id}')" class="flex items-center justify-between p-4 rounded-none cursor-pointer transition-all border-2 ${
                        active ? "bg-[#2a292e] border-[#e50914] shadow-[4px_4px_0px_#e50914]" : "bg-[#131317] border-white/70 hover:bg-[#2a292e] shadow-[4px_4px_0px_#000000]"
                      }">
                        <div class="flex items-center gap-3.5 min-w-0">
                          <div class="w-11 h-11 rounded-none bg-[#131317] border-2 ${active ? "border-white text-[#e50914]" : "border-white/70 text-white"} flex items-center justify-center shrink-0">
                            <span class="material-symbols-outlined">${m.icon}</span>
                          </div>
                          <div class="min-w-0">
                            <div class="flex items-center gap-2 flex-wrap">
                              <span class="text-sm font-extrabold text-white">${m.title}</span>
                              ${m.badge ? `<span class="bg-[#e50914] text-white text-[10px] font-extrabold uppercase px-2 py-0.5 rounded-none border border-white">${m.badge}</span>` : ""}
                            </div>
                            <p class="text-xs text-slate-300 mt-0.5">${m.sub}</p>
                          </div>
                        </div>
                        <div class="w-5 h-5 rounded-none border-2 ${active ? "border-white bg-[#e50914]" : "border-white/70 bg-[#1f1f23]"} flex items-center justify-center shrink-0 ml-3">
                          <span class="w-2.5 h-2.5 rounded-none ${active ? "bg-white" : "bg-transparent"}"></span>
                        </div>
                      </div>
                    `;
                  })
                  .join("")}
              </div>
            </section>
          </div>

          <aside class="lg:col-span-5 lg:sticky lg:top-6 bg-[#1f1f23] rounded-none border-[3px] border-white p-6 shadow-[6px_6px_0px_#e50914]">
            <div class="flex items-center justify-between mb-4">
              <span class="text-xs font-extrabold uppercase tracking-widest text-slate-200">Ringkasan Tiket</span>
              <span class="bg-[#131317] text-white border-2 border-white text-[11px] font-extrabold uppercase px-2.5 py-0.5 rounded-none">${escapeHtml(s.studio?.studio_type || "REGULAR")}</span>
            </div>

            <div class="flex gap-4 bg-[#131317] border-2 border-white/80 p-4 rounded-none mb-5 shadow-[4px_4px_0px_#000000]">
              <img src="${posterFor(s.movie_id)}" class="w-20 h-28 rounded-none border-2 border-white object-cover shrink-0"/>
              <div class="min-w-0">
                <h3 class="text-base font-extrabold text-white uppercase tracking-tight">${escapeHtml(s.movie?.title || "")}</h3>
                <p class="text-xs text-slate-300 mt-1">${escapeHtml(s.studio?.cinema?.name || "")} • ${escapeHtml(s.studio?.name || "")}</p>
                <p class="text-xs font-bold text-white mt-2">${formatDateID(s.start_time)} • ${formatTimeWIB(s.start_time)}</p>
                <p class="text-xs font-extrabold text-[#e50914] mt-1">Kursi: ${escapeHtml(seatLabels)} (${count} Tiket)</p>
              </div>
            </div>

            <div class="flex flex-col gap-2.5 py-4 px-4 bg-[#131317] border-2 border-white/80 rounded-none mb-6 text-sm shadow-[4px_4px_0px_#000000]">
              <div class="flex justify-between"><span class="text-slate-300">Harga Tiket</span><span class="tabular-nums font-bold text-white">${formatRupiah(baseTotal)}</span></div>
              <div class="flex justify-between"><span class="text-slate-300">Biaya Layanan</span><span class="tabular-nums font-bold text-white">${formatRupiah(fee)}</span></div>
              <div class="flex justify-between"><span class="text-slate-300">Potongan Promo</span><span class="tabular-nums font-bold text-emerald-400">-${formatRupiah(discount)}</span></div>
              <div class="h-[2px] bg-white/30 my-1"></div>
              <div class="flex justify-between items-center"><span class="font-extrabold text-white uppercase">Total Bayar</span><span class="text-2xl font-black text-[#e50914] tabular-nums">${formatRupiah(finalTotal)}</span></div>
            </div>

            <div class="flex flex-col gap-3">
              <button onclick="window.app.payNow()" class="w-full bg-[#e50914] hover:bg-[#c0000c] text-white text-sm font-extrabold uppercase tracking-wider py-3.5 rounded-none border-[3px] border-white shadow-[5px_5px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all flex items-center justify-center gap-2">
                <span class="material-symbols-outlined text-lg">lock</span>
                <span>Bayar Sekarang</span>
              </button>
              <button onclick="window.app.cancelPendingBooking(${b.id})" class="w-full bg-[#131317] hover:bg-rose-950 text-slate-200 hover:text-white text-xs font-extrabold uppercase tracking-wider py-3 rounded-none border-2 border-white/70 shadow-[3px_3px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all">
                Batalkan Pesanan &amp; Lepas Kursi
              </button>
            </div>
          </aside>
        </div>
      </div>
    `;

    startCheckoutCountdown(b.expired_at);
  }

  function startCheckoutCountdown(expiredAtIso) {
    stopCountdown();
    const tick = () => {
      const el = document.getElementById("checkout-timer");
      if (!el) return stopCountdown();
      const diffSec = Math.max(0, Math.floor((new Date(expiredAtIso).getTime() - Date.now()) / 1000));
      const m = String(Math.floor(diffSec / 60)).padStart(2, "0");
      const s = String(diffSec % 60).padStart(2, "0");
      el.textContent = `${m}:${s}`;
      if (diffSec <= 0) {
        stopCountdown();
        showToast("Waktu bayar 10 menit habis, kursi otomatis dibuka kembali");
        refreshMyBookings().then(() => navigate("tiket-saya"));
      }
    };
    tick();
    state.countdownInterval = setInterval(tick, 1000);
  }

  async function payNow() {
    if (!state.activeBooking) return;
    try {
      const res = await api(`/api/v1/bookings/${state.activeBooking.id}/pay`, {
        method: "POST",
        body: JSON.stringify({
          payment_method: state.paymentMethod,
          promo_code: state.promoCode,
        }),
      });
      state.activeBooking = res.data;
      await refreshMyBookings();
      showToast(res.message);
      navigate("etiket");
    } catch (err) {
      showToast(err.message);
    }
  }

  async function cancelPendingBooking(bookingId) {
    try {
      const res = await api(`/api/v1/bookings/${bookingId}/cancel`, { method: "POST" });
      await refreshMyBookings();
      showToast(res.message);
      navigate("bioskop");
    } catch (err) {
      showToast(err.message);
    }
  }

  async function cancelPaidBooking(bookingId) {
    if (!window.confirm("Batalkan tiket lunas ini? Dana akan dikembalikan 100% dan kursi otomatis tersedia lagi untuk pembeli lain.")) {
      return;
    }
    try {
      const res = await api(`/api/v1/bookings/${bookingId}/cancel`, { method: "POST" });
      state.activeBooking = res.data;
      await refreshMyBookings();
      showToast(res.message);
      renderChrome();
      renderView();
    } catch (err) {
      showToast(err.message);
    }
  }

  // ==================== VIEW 5: E-TIKET RESMI (NEO-BRUTALISM) ====================
  function renderETicket(root) {
    const b = state.activeBooking;
    if (!b) return navigate("tiket-saya");

    const s = b.schedule || {};
    const seatLabels = (b.seats || []).map((bs) => `${bs.seat?.row_label || ""}${bs.seat?.seat_number || ""}`).join(", ");
    const isRefunded = b.status === "REFUNDED";
    const isCustomerRefund = isRefunded && b.refund?.initiated_by === "CUSTOMER";
    const statusBadge = isRefunded
      ? `<span class="px-3 py-1 rounded-none border-2 border-white bg-purple-700 text-white text-xs font-extrabold uppercase flex items-center gap-1 shadow-[3px_3px_0px_#000000]"><i class="fa-solid fa-rotate-left"></i> REFUND 100%</span>`
      : b.status === "PAID"
      ? `<span class="px-3 py-1 rounded-none border-2 border-white bg-[#e50914] text-white text-xs font-extrabold uppercase flex items-center gap-1.5 shadow-[3px_3px_0px_#000000]"><span class="material-symbols-outlined text-sm">verified</span> LUNAS</span>`
      : `<span class="px-3 py-1 rounded-none border-2 border-white bg-amber-400 text-black text-xs font-extrabold uppercase shadow-[3px_3px_0px_#000000]">${escapeHtml(b.status)}</span>`;

    root.innerHTML = `
      <div class="relative w-full max-w-4xl mx-auto flex flex-col items-center py-2">
        <section class="flex flex-col items-center text-center mb-8">
          <div class="w-16 h-16 rounded-none border-2 border-white ${isRefunded ? "bg-purple-700" : "bg-[#e50914]"} text-white flex items-center justify-center shadow-[4px_4px_0px_#000000] mb-4">
            <span class="material-symbols-outlined text-3xl">${isRefunded ? "currency_exchange" : "check_circle"}</span>
          </div>
          <h1 class="text-2xl sm:text-3xl font-black text-white uppercase tracking-tight mb-1.5">
            ${
              isCustomerRefund
                ? "Tiket Dibatalkan • Dana Dikembalikan & Kursi Direstok"
                : isRefunded
                ? "Jadwal Dibatalkan Bioskop • Dana Dikembalikan"
                : "Pembayaran Berhasil! Tiket Anda Siap"
            }
          </h1>
          <p class="text-xs sm:text-sm text-slate-300 max-w-md">
            ${
              isRefunded
                ? `Alasan: "${escapeHtml(b.refund?.reason || s.cancellation_reason || "sedang ada kendala")}". Pengembalian penuh sebesar ${formatRupiah(b.refund?.refund_amount || b.total_amount)} telah diproses.`
                : "Tunjukkan kode QR atau kode booking Zanema di bawah ini kepada petugas di pintu masuk studio."
            }
          </p>
        </section>

        <section class="w-full">
          <div class="flex flex-col lg:flex-row bg-[#131317] rounded-none border-[3px] border-white shadow-[8px_8px_0px_#e50914] overflow-hidden">
            <div class="flex-1 p-6 sm:p-8 flex flex-col justify-between bg-[#131317]">
              <div class="flex flex-col sm:flex-row gap-6 items-start">
                <div class="w-24 sm:w-28 aspect-[2/3] rounded-none border-2 border-white overflow-hidden shadow-[4px_4px_0px_#000000] shrink-0 bg-[#0e0e12]">
                  <img src="${posterFor(s.movie_id)}" class="w-full h-full object-cover"/>
                </div>
                <div class="flex-1 min-w-0">
                  <div class="flex flex-wrap items-center gap-2.5 mb-3">
                    <span class="px-3 py-1 rounded-none border-2 border-white bg-[#e50914] text-white text-xs font-extrabold uppercase tracking-wider shadow-[3px_3px_0px_#000000]">${escapeHtml(s.studio?.studio_type || "REGULAR")}</span>
                    <span class="px-3 py-1 rounded-none border-2 border-white bg-[#1f1f23] text-white text-xs font-bold uppercase tracking-wider shadow-[3px_3px_0px_#000000]">${s.movie?.duration_minutes || "-"} Menit</span>
                  </div>
                  <h2 class="text-xl sm:text-2xl font-black text-white uppercase tracking-tight mb-1">${escapeHtml(s.movie?.title || "")}</h2>
                  <p class="text-xs font-bold text-slate-300 uppercase tracking-wider mb-4">${escapeHtml(s.movie?.genre || "")} • ${escapeHtml(s.movie?.rating || "SU")}</p>
                  <div class="flex items-start gap-2.5 bg-[#1b1b1f] p-3.5 rounded-none border-2 border-white shadow-[4px_4px_0px_#000000]">
                    <span class="material-symbols-outlined text-[#e50914] text-lg">location_on</span>
                    <div>
                      <p class="text-sm font-extrabold text-white">${escapeHtml(s.studio?.cinema?.name || "")}</p>
                      <p class="text-xs text-slate-300">${escapeHtml(s.studio?.name || "")} • ${escapeHtml(s.studio?.cinema?.address || "")}</p>
                    </div>
                  </div>
                </div>
              </div>

              <div class="grid grid-cols-2 sm:grid-cols-3 gap-4 mt-7 bg-[#1b1b1f] p-4 rounded-none border-2 border-white shadow-[4px_4px_0px_#000000]">
                <div>
                  <p class="text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">Tanggal</p>
                  <p class="text-sm font-extrabold text-white">${formatDateID(s.start_time)}</p>
                </div>
                <div>
                  <p class="text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">Jam Tayang</p>
                  <p class="text-sm font-extrabold text-white">${formatTimeWIB(s.start_time)}</p>
                </div>
                <div class="col-span-2 sm:col-span-1">
                  <p class="text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">Kursi</p>
                  <p class="text-sm font-extrabold text-[#e50914]">${escapeHtml(seatLabels)} <span class="text-xs font-semibold text-slate-300">(${(b.seats || []).length} Tiket)</span></p>
                </div>
              </div>
            </div>

            <!-- Neo-Brutalism Square-Dot Ticket Divider -->
            <div class="relative flex lg:flex-col items-center justify-center bg-[#1b1b1f] border-y-[3px] lg:border-y-0 lg:border-x-[3px] border-white lg:w-6 py-2 lg:py-4 px-4 lg:px-0">
              <div class="flex lg:flex-col items-center justify-center gap-2.5 overflow-hidden">
                ${Array(8).fill('<span class="w-2 h-2 rounded-none bg-white"></span>').join("")}
              </div>
            </div>

            <!-- QR & Booking Code -->
            <div class="w-full lg:w-80 p-6 sm:p-8 bg-[#1b1b1f] flex flex-col items-center justify-between text-center">
              <div class="w-full flex items-center justify-between mb-4">
                <span class="text-[11px] font-extrabold text-slate-300 uppercase tracking-widest">E-Tiket Resmi</span>
                ${statusBadge}
              </div>

              <div class="p-4 rounded-none bg-white text-black border-[3px] border-black shadow-[6px_6px_0px_#e50914] mb-5">
                <svg aria-label="QR Code Tiket Zanema" class="w-32 h-32" fill="currentColor" viewBox="0 0 100 100">
                  <path d="M10,10 h25 v25 h-25 z M15,15 v15 h15 v-15 z M20,20 h5 v5 h-5 z"></path>
                  <path d="M65,10 h25 v25 h-25 z M70,15 v15 h15 v-15 z M75,20 h5 v5 h-5 z"></path>
                  <path d="M10,65 h25 v25 h-25 z M15,70 v15 h15 v-15 z M20,75 h5 v5 h-5 z"></path>
                  <rect x="42" y="12" width="8" height="8"></rect><rect x="54" y="20" width="6" height="12"></rect>
                  <rect x="12" y="44" width="14" height="6"></rect><rect x="42" y="42" width="16" height="16"></rect>
                  <rect x="66" y="44" width="18" height="8"></rect><rect x="44" y="66" width="8" height="18"></rect>
                  <rect x="60" y="70" width="12" height="8"></rect><rect x="78" y="76" width="12" height="12"></rect>
                </svg>
              </div>

              <div class="w-full">
                <p class="text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1.5">Kode Booking</p>
                <button type="button" onclick="window.app.copyText('${escapeHtml(b.booking_code)}')" class="w-full py-2.5 px-4 rounded-none bg-[#131317] hover:bg-[#2a292e] border-2 border-white text-white text-base font-black tracking-widest shadow-[4px_4px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all flex items-center justify-center gap-2">
                  <span>${escapeHtml(b.booking_code)}</span>
                  <span class="material-symbols-outlined text-base text-[#e50914]">content_copy</span>
                </button>
              </div>
            </div>
          </div>
        </section>

        <section class="no-print flex flex-wrap items-center justify-center gap-3.5 mt-8">
          <button onclick="window.print()" class="px-6 py-3 rounded-none bg-[#e50914] hover:bg-[#c0000c] text-white text-xs font-extrabold uppercase tracking-wider border-2 border-white shadow-[4px_4px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all flex items-center gap-2">
            <span class="material-symbols-outlined text-base">print</span>
            <span>Unduh E-Tiket (PDF)</span>
          </button>
          ${
            b.status === "PAID" && new Date(s.start_time) > new Date()
              ? `<button onclick="window.app.cancelPaidBooking(${b.id})" class="px-6 py-3 rounded-none bg-[#1f1f23] hover:bg-rose-950 border-2 border-[#e50914] text-rose-300 hover:text-white text-xs font-extrabold uppercase tracking-wider shadow-[4px_4px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all">Batalkan Tiket &amp; Restok Kursi</button>`
              : ""
          }
          <button onclick="window.app.navigate('tiket-saya')" class="px-6 py-3 rounded-none bg-[#1f1f23] hover:bg-[#2a292e] text-white text-xs font-extrabold uppercase tracking-wider border-2 border-white shadow-[4px_4px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all">
            Semua Tiket Saya
          </button>
        </section>
      </div>
    `;
  }

  // ==================== VIEW 5B: DAFTAR TIKET SAYA ====================
  async function renderMyTickets(root) {
    if (!state.token) {
      root.innerHTML = `
        <div class="bg-[#1f1f23] rounded-none border-[3px] border-white p-10 text-center max-w-md mx-auto shadow-[8px_8px_0px_#e50914]">
          <i class="fa-solid fa-ticket text-3xl text-[#e50914] mb-3"></i>
          <h2 class="text-lg font-extrabold uppercase text-white mb-1">Masuk untuk Melihat Tiket Anda</h2>
          <p class="text-xs text-slate-300 mb-5">Semua tiket aktif, pesanan menunggu pembayaran, maupun riwayat pengembalian dana tersimpan di sini.</p>
          <button onclick="window.app.openAuthModal('login')" class="px-6 py-2.5 rounded-none bg-[#e50914] hover:bg-[#c0000c] text-white text-xs font-extrabold uppercase tracking-wider border-2 border-white shadow-[4px_4px_0px_#000000]">Masuk / Daftar</button>
        </div>
      `;
      return;
    }

    await refreshMyBookings();
    renderChrome();

    if (state.myBookings.length === 0) {
      root.innerHTML = `
        <div class="bg-[#1f1f23] rounded-none border-[3px] border-white p-10 text-center max-w-md mx-auto shadow-[8px_8px_0px_#e50914]">
          <h2 class="text-lg font-extrabold uppercase text-white mb-1">Belum Ada Pesanan Tiket</h2>
          <p class="text-xs text-slate-300 mb-5">Yuk pilih film dan kursi favorit Anda sekarang di Zanema.</p>
          <button onclick="window.app.navigate('bioskop')" class="px-6 py-2.5 rounded-none bg-[#e50914] hover:bg-[#c0000c] text-white text-xs font-extrabold uppercase tracking-wider border-2 border-white shadow-[4px_4px_0px_#000000]">Lihat Jadwal Tayang</button>
        </div>
      `;
      return;
    }

    const itemsHtml = state.myBookings
      .map((b) => {
        const s = b.schedule || {};
        const seats = (b.seats || []).map((bs) => `${bs.seat?.row_label || ""}${bs.seat?.seat_number || ""}`).join(", ");
        const statusStyles = {
          PAID: "bg-emerald-500 text-black border-black",
          PENDING: "bg-amber-400 text-black border-black",
          EXPIRED: "bg-[#2a292e] text-slate-300 border-white/50",
          CANCELLED: "bg-rose-700 text-white border-white",
          REFUNDED: "bg-purple-700 text-white border-white",
        }[b.status] || "bg-[#2a292e] text-white border-white";

        const statusText = {
          PAID: "LUNAS",
          PENDING: "MENUNGGU BAYAR",
          EXPIRED: "KEDALUWARSA (KURSI DILEPAS)",
          CANCELLED: "DIBATALKAN",
          REFUNDED: "DANA DIKEMBALIKAN 100%",
        }[b.status] || b.status;

        const actionBtn =
          b.status === "PENDING"
            ? `<button onclick="window.app.openBookingCheckout(${b.id})" class="px-4 py-2 rounded-none bg-[#e50914] hover:bg-[#c0000c] text-white text-xs font-extrabold uppercase border-2 border-white shadow-[3px_3px_0px_#000000]">Lanjut Bayar</button>`
            : b.status === "PAID"
            ? `<div class="flex flex-wrap gap-2">
                <button onclick="window.app.openBookingTicket(${b.id})" class="px-4 py-2 rounded-none bg-[#2a292e] hover:bg-[#39393d] text-white text-xs font-extrabold uppercase border-2 border-white shadow-[3px_3px_0px_#000000]">Buka E-Tiket</button>
                ${new Date(s.start_time) > new Date() ? `<button onclick="window.app.cancelPaidBooking(${b.id})" class="px-4 py-2 rounded-none bg-[#131317] hover:bg-rose-950 border-2 border-[#e50914] text-rose-300 hover:text-white text-xs font-extrabold uppercase shadow-[3px_3px_0px_#000000]">Batalkan &amp; Restok</button>` : ""}
              </div>`
            : b.status === "REFUNDED"
            ? `<button onclick="window.app.openBookingTicket(${b.id})" class="px-4 py-2 rounded-none bg-[#2a292e] hover:bg-[#39393d] text-white text-xs font-extrabold uppercase border-2 border-white shadow-[3px_3px_0px_#000000]">Buka E-Tiket</button>`
            : "";

        return `
          <div class="bg-[#1f1f23] rounded-none border-2 border-white/90 p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4 shadow-[6px_6px_0px_#000000]">
            <div class="flex items-start gap-4">
              <img src="${posterFor(s.movie_id)}" class="w-14 h-20 rounded-none border-2 border-white object-cover shrink-0 shadow-[3px_3px_0px_#000000]"/>
              <div>
                <div class="flex flex-wrap items-center gap-2 mb-1.5">
                  <span class="text-xs font-mono font-extrabold text-slate-300">${escapeHtml(b.booking_code)}</span>
                  <span class="text-[10px] font-extrabold uppercase px-2 py-0.5 rounded-none border ${statusStyles}">${statusText}</span>
                </div>
                <h3 class="text-base font-extrabold text-white uppercase tracking-tight">${escapeHtml(s.movie?.title || "Film")}</h3>
                <p class="text-xs text-slate-300 mt-0.5">${escapeHtml(s.studio?.cinema?.name || "")} (${escapeHtml(s.studio?.name || "")}) • ${formatDateID(s.start_time)} ${formatTimeWIB(s.start_time)}</p>
                <p class="text-xs text-[#e50914] font-extrabold mt-1">Kursi: ${escapeHtml(seats || "-")} • Total: ${formatRupiah(b.total_amount)}</p>
                ${b.refund ? `<p class="text-[11px] font-bold text-purple-300 mt-1">Refund: ${formatRupiah(b.refund.refund_amount)} (${escapeHtml(b.refund.reason)})</p>` : ""}
              </div>
            </div>
            <div class="shrink-0">${actionBtn}</div>
          </div>
        `;
      })
      .join("");

    root.innerHTML = `
      <div class="w-full max-w-4xl mx-auto flex flex-col gap-5">
        <h1 class="text-2xl font-black text-white uppercase tracking-tight">Riwayat &amp; E-Tiket Saya</h1>
        ${itemsHtml}
      </div>
    `;
  }

  // ==================== VIEW 6: PANEL ADMIN (CRUD JADWAL TAYANG) ====================
  function renderAdminPanel(root) {
    if (!state.user || state.user.role !== "ADMIN") {
      return navigate("katalog");
    }

    const editing = state.editingScheduleId
      ? state.schedules.find((s) => s.id === state.editingScheduleId)
      : null;

    const movieOpts = state.movies
      .map((m) => `<option value="${m.id}" ${editing?.movie_id === m.id ? "selected" : ""}>${escapeHtml(m.title)} (${m.duration_minutes} mnt)</option>`)
      .join("");
    const studioOpts = state.studios
      .map(
        (st) => `<option value="${st.id}" ${editing?.studio_id === st.id ? "selected" : ""}>${escapeHtml(st.cinema?.name || "")} - ${escapeHtml(st.name)} (${escapeHtml(st.studio_type)})</option>`
      )
      .join("");

    const defaultStart = editing
      ? new Date(new Date(editing.start_time).getTime() - new Date().getTimezoneOffset() * 60000).toISOString().slice(0, 16)
      : "2026-12-02T14:00";

    const rowsHtml = state.schedules
      .map((s) => {
        const badgeCls =
          s.status === "SCHEDULED"
            ? "bg-emerald-500 text-black border-black"
            : s.status === "CANCELLED"
            ? "bg-[#e50914] text-white border-white"
            : "bg-[#2a292e] text-slate-300 border-white/50";

        const statusLabel =
          s.status === "SCHEDULED" ? "Aktif" : s.status === "CANCELLED" ? "Dibatalkan" : "Selesai";

        return `
          <tr class="border-b-2 border-white/15 text-xs">
            <td class="py-3 px-3 font-mono font-bold text-slate-400">#${s.id}</td>
            <td class="py-3 px-3 font-extrabold text-white">${escapeHtml(s.movie?.title || "")}</td>
            <td class="py-3 px-3 text-slate-300">${escapeHtml(s.studio?.cinema?.name || "")} • ${escapeHtml(s.studio?.name || "")}</td>
            <td class="py-3 px-3 text-slate-200 tabular-nums">${formatDateID(s.start_time)}<br/><span class="text-slate-400">${formatTimeWIB(s.start_time)} - ${formatTimeWIB(s.end_time)}</span></td>
            <td class="py-3 px-3 font-extrabold text-white tabular-nums">${formatRupiah(s.price)}</td>
            <td class="py-3 px-3">
              <span class="px-2 py-0.5 rounded-none border text-[10px] font-extrabold uppercase ${badgeCls}">${statusLabel}</span>
              ${s.cancellation_reason ? `<div class="text-[10px] text-rose-300 font-semibold mt-1">${escapeHtml(s.cancellation_reason)}</div>` : ""}
            </td>
            <td class="py-3 px-3 text-right space-x-1.5 whitespace-nowrap">
              ${
                s.status === "SCHEDULED"
                  ? `
                <button onclick="window.app.editSchedule(${s.id})" class="px-2.5 py-1 rounded-none bg-[#2a292e] hover:bg-[#39393d] border border-white text-white font-bold uppercase shadow-[2px_2px_0px_#000000]">Ubah</button>
                <button onclick="window.app.promptCancelSchedule(${s.id})" class="px-2.5 py-1 rounded-none bg-amber-400 hover:bg-amber-300 border border-black text-black font-extrabold uppercase shadow-[2px_2px_0px_#000000]">Batalkan</button>
              `
                  : ""
              }
              <button onclick="window.app.deleteSchedule(${s.id})" class="px-2.5 py-1 rounded-none bg-[#e50914] hover:bg-[#c0000c] border border-white text-white font-extrabold uppercase shadow-[2px_2px_0px_#000000]">Hapus</button>
            </td>
          </tr>
        `;
      })
      .join("");

    root.innerHTML = `
      <div class="w-full flex flex-col gap-6">
        <div class="flex items-center justify-between">
          <div>
            <span class="text-xs font-extrabold uppercase tracking-widest text-amber-400">Panel Admin Zanema</span>
            <h1 class="text-2xl font-black text-white uppercase tracking-tight mt-0.5">Kelola Jadwal Tayang</h1>
          </div>
        </div>

        <form onsubmit="window.app.submitScheduleForm(event)" class="bg-[#1f1f23] rounded-none border-[3px] border-white p-6 shadow-[6px_6px_0px_#e50914] grid grid-cols-1 md:grid-cols-12 gap-4 items-end">
          <div class="md:col-span-3">
            <label class="block text-xs font-extrabold uppercase tracking-wider text-slate-200 mb-1">Film</label>
            <select id="adm-movie" required class="w-full bg-[#131317] text-xs font-bold text-white px-3 py-2.5 rounded-none border-2 border-white/80 shadow-[3px_3px_0px_#000000] focus:outline-none focus:border-[#e50914] focus:ring-0">${movieOpts}</select>
          </div>
          <div class="md:col-span-4">
            <label class="block text-xs font-extrabold uppercase tracking-wider text-slate-200 mb-1">Bioskop &amp; Studio</label>
            <select id="adm-studio" required class="w-full bg-[#131317] text-xs font-bold text-white px-3 py-2.5 rounded-none border-2 border-white/80 shadow-[3px_3px_0px_#000000] focus:outline-none focus:border-[#e50914] focus:ring-0">${studioOpts}</select>
          </div>
          <div class="md:col-span-3">
            <label class="block text-xs font-extrabold uppercase tracking-wider text-slate-200 mb-1">Waktu Mulai (WIB)</label>
            <input id="adm-start" type="datetime-local" required value="${defaultStart}" class="w-full bg-[#131317] text-xs font-bold text-white px-3 py-2.5 rounded-none border-2 border-white/80 shadow-[3px_3px_0px_#000000] focus:outline-none focus:border-[#e50914] focus:ring-0"/>
          </div>
          <div class="md:col-span-2">
            <label class="block text-xs font-extrabold uppercase tracking-wider text-slate-200 mb-1">Harga (Rp)</label>
            <input id="adm-price" type="number" min="0" step="1000" required value="${editing ? editing.price : 50000}" class="w-full bg-[#131317] text-xs font-bold text-white px-3 py-2.5 rounded-none border-2 border-white/80 shadow-[3px_3px_0px_#000000] focus:outline-none focus:border-[#e50914] focus:ring-0"/>
          </div>
          <div class="md:col-span-12 flex items-center justify-end gap-3 pt-2">
            ${
              editing
                ? `<button type="button" onclick="window.app.cancelEditSchedule()" class="px-4 py-2.5 rounded-none bg-[#131317] hover:bg-[#2a292e] border-2 border-white text-xs font-extrabold uppercase text-slate-200 shadow-[3px_3px_0px_#000000]">Batal Ubah</button>`
                : ""
            }
            <button type="submit" class="px-6 py-2.5 rounded-none bg-[#e50914] hover:bg-[#c0000c] border-2 border-white text-xs font-extrabold uppercase tracking-wider text-white shadow-[4px_4px_0px_#000000] active:translate-x-[2px] active:translate-y-[2px] active:shadow-none transition-all">
              ${editing ? `Simpan Perubahan #${editing.id}` : "+ Tambah Jadwal Tayang"}
            </button>
          </div>
        </form>

        <div class="bg-[#1f1f23] rounded-none border-[3px] border-white p-5 shadow-[6px_6px_0px_#000000] overflow-x-auto">
          <table class="w-full text-left border-collapse">
            <thead>
              <tr class="border-b-2 border-white/60 text-[11px] font-extrabold uppercase tracking-wider text-slate-300">
                <th class="py-2.5 px-3">ID</th>
                <th class="py-2.5 px-3">Judul Film</th>
                <th class="py-2.5 px-3">Bioskop &amp; Studio</th>
                <th class="py-2.5 px-3">Jam Tayang</th>
                <th class="py-2.5 px-3">Harga</th>
                <th class="py-2.5 px-3">Status</th>
                <th class="py-2.5 px-3 text-right">Aksi</th>
              </tr>
            </thead>
            <tbody>${rowsHtml}</tbody>
          </table>
        </div>
      </div>
    `;
  }

  async function submitScheduleForm(ev) {
    ev.preventDefault();
    const movieId = Number(document.getElementById("adm-movie").value);
    const studioId = Number(document.getElementById("adm-studio").value);
    const rawStart = document.getElementById("adm-start").value;
    const price = Number(document.getElementById("adm-price").value);
    const startIso = rawStart.length === 16 ? `${rawStart}:00+07:00` : rawStart;

    try {
      if (state.editingScheduleId) {
        const movie = state.movies.find((m) => m.id === movieId);
        const endDate = new Date(new Date(startIso).getTime() + (movie?.duration_minutes || 120) * 60000);
        const res = await api(`/api/v1/schedules/${state.editingScheduleId}`, {
          method: "PUT",
          body: JSON.stringify({
            movie_id: movieId,
            studio_id: studioId,
            start_time: startIso,
            end_time: endDate.toISOString(),
            price,
          }),
        });
        state.editingScheduleId = null;
        showToast(res.message);
      } else {
        const res = await api("/api/v1/schedules", {
          method: "POST",
          body: JSON.stringify({
            movie_id: movieId,
            studio_id: studioId,
            start_time: startIso,
            price,
          }),
        });
        showToast(res.message);
      }
      await refreshSchedules();
      renderView();
    } catch (err) {
      showToast(err.message);
    }
  }

  async function promptCancelSchedule(id) {
    const reason = window.prompt("Masukkan alasan pembatalan jadwal (pesanan lunas akan di-refund 100%):", "sedang ada kendala");
    if (!reason) return;
    try {
      const res = await api(`/api/v1/schedules/${id}/cancel`, {
        method: "PATCH",
        body: JSON.stringify({ cancellation_reason: reason }),
      });
      showToast(res.message);
      await refreshSchedules();
      renderView();
    } catch (err) {
      showToast(err.message);
    }
  }

  async function deleteSchedule(id) {
    if (!window.confirm(`Hapus jadwal #${id}?`)) return;
    try {
      const res = await api(`/api/v1/schedules/${id}`, { method: "DELETE" });
      showToast(res.message);
      await refreshSchedules();
      renderView();
    } catch (err) {
      showToast(err.message);
    }
  }

  // ==================== AUTH MODAL HANDLERS ====================
  function openAuthModal(mode = "login") {
    switchAuthTab(mode);
    document.getElementById("auth-modal").classList.remove("hidden");
  }

  function closeAuthModal() {
    document.getElementById("auth-modal").classList.add("hidden");
    document.getElementById("auth-error").classList.add("hidden");
  }

  function switchAuthTab(mode) {
    state.authMode = mode;
    const isReg = mode === "register";
    document.getElementById("auth-modal-title").textContent = isReg ? "Daftar Akun Pelanggan" : "Masuk ke Akun Anda";
    document.getElementById("field-fullname").classList.toggle("hidden", !isReg);
    document.getElementById("field-phone").classList.toggle("hidden", !isReg);
    document.getElementById("demo-accounts-box").classList.toggle("hidden", isReg);
    document.getElementById("auth-submit-btn").textContent = isReg ? "Daftar Sekarang" : "Masuk Sekarang";

    document.getElementById("tab-login").className = !isReg
      ? "py-2.5 text-xs font-extrabold uppercase tracking-wider rounded-none bg-[#e50914] text-white border-2 border-white shadow-[3px_3px_0px_#000000] transition-all"
      : "py-2.5 text-xs font-bold uppercase tracking-wider rounded-none bg-[#131317] text-slate-300 hover:text-white border-2 border-white/60 shadow-[3px_3px_0px_#000000] transition-all";
    document.getElementById("tab-register").className = isReg
      ? "py-2.5 text-xs font-extrabold uppercase tracking-wider rounded-none bg-[#e50914] text-white border-2 border-white shadow-[3px_3px_0px_#000000] transition-all"
      : "py-2.5 text-xs font-bold uppercase tracking-wider rounded-none bg-[#131317] text-slate-300 hover:text-white border-2 border-white/60 shadow-[3px_3px_0px_#000000] transition-all";
    document.getElementById("auth-error").classList.add("hidden");
  }

  function fillDemo(email, password) {
    switchAuthTab("login");
    document.getElementById("auth-email").value = email;
    document.getElementById("auth-password").value = password;
  }

  document.getElementById("auth-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    const errEl = document.getElementById("auth-error");
    errEl.classList.add("hidden");

    const email = document.getElementById("auth-email").value.trim();
    const password = document.getElementById("auth-password").value;

    try {
      if (state.authMode === "register") {
        const fullName = document.getElementById("auth-fullname").value.trim();
        const phone = document.getElementById("auth-phone").value.trim();
        await api("/api/v1/auth/register", {
          method: "POST",
          body: JSON.stringify({ full_name: fullName, email, password, phone: phone || undefined }),
        });
      }

      const loginRes = await api("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password }),
      });
      state.token = loginRes.data.token;
      state.user = loginRes.data.user;
      localStorage.setItem("cine_token", state.token);
      localStorage.setItem("cine_user", JSON.stringify(state.user));

      await refreshMyBookings();
      closeAuthModal();
      renderChrome();
      showToast(`Selamat datang, ${state.user.full_name}!`);

      if (state.afterLoginCallback) {
        const cb = state.afterLoginCallback;
        state.afterLoginCallback = null;
        cb();
      } else {
        renderView();
      }
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove("hidden");
    }
  });

  function logout(silent = false) {
    state.token = "";
    state.user = null;
    state.myBookings = [];
    localStorage.removeItem("cine_token");
    localStorage.removeItem("cine_user");
    renderChrome();
    if (!silent) {
      showToast("Anda telah keluar dari akun");
      if (state.view === "admin" || state.view === "checkout") navigate("katalog");
      else renderView();
    }
  }

  document.querySelectorAll(".nav-pill").forEach((btn) => {
    btn.addEventListener("click", () => navigate(btn.dataset.nav));
  });

  document.getElementById("global-search").addEventListener("input", (e) => {
    state.searchQuery = e.target.value;
    if (state.view !== "bioskop") navigate("bioskop");
    else renderView();
  });

  window.app = {
    navigate,
    scrollThumbnails(offset) {
      document.getElementById("movie-thumbnails")?.scrollBy({ left: offset, behavior: "smooth" });
    },
    selectMovie(id) {
      state.selectedMovieId = id;
      if (state.view !== "katalog") navigate("katalog");
      else {
        renderChrome();
        renderView();
      }
    },
    buyForMovie(movieId) {
      const first = state.schedules.find((s) => s.movie_id === movieId && s.status === "SCHEDULED");
      if (first) openSeatMap(first.id);
      else navigate("bioskop");
    },
    setFilter(kind, val) {
      if (kind === "city") state.filterCity = val;
      if (kind === "format") state.filterFormat = val;
      if (kind === "date") state.filterDate = val;
      if (kind === "search") state.searchQuery = val;
      renderView();
    },
    openSeatMap,
    toggleSeat(seatId) {
      if (state.selectedSeatIds.has(seatId)) state.selectedSeatIds.delete(seatId);
      else {
        if (state.selectedSeatIds.size >= 8) return showToast("Maksimal 8 kursi dalam satu pesanan");
        state.selectedSeatIds.add(seatId);
      }
      renderView();
    },
    lockSeatsAndCheckout,
    applyPromo(code) {
      state.promoCode = (code || "").trim().toUpperCase();
      if (state.promoCode === "DISKON25") showToast("Kode DISKON25 diterapkan! Potongan Rp 25.000");
      else if (state.promoCode) showToast("Kode promo tidak ditemukan");
      renderView();
    },
    setPaymentMethod(m) {
      state.paymentMethod = m;
      renderView();
    },
    payNow,
    cancelPendingBooking,
    cancelPaidBooking,
    openBookingCheckout(id) {
      const b = state.myBookings.find((x) => x.id === id);
      if (!b) return;
      state.activeBooking = b;
      navigate("checkout");
    },
    openBookingTicket(id) {
      const b = state.myBookings.find((x) => x.id === id);
      if (!b) return;
      state.activeBooking = b;
      navigate("etiket");
    },
    copyText(txt) {
      navigator.clipboard?.writeText(txt).catch(() => {});
      showToast(`Kode ${txt} berhasil disalin`);
    },
    editSchedule(id) {
      state.editingScheduleId = id;
      renderView();
    },
    cancelEditSchedule() {
      state.editingScheduleId = null;
      renderView();
    },
    submitScheduleForm,
    promptCancelSchedule,
    deleteSchedule,
    openAuthModal,
    closeAuthModal,
    switchAuthTab,
    fillDemo,
    logout,
  };

  loadInitialData();
})();
