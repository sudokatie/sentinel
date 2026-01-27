// Sentinel - Signal Theme

const theme = {
    bg: '#12141c',
    surface: '#1a1d28',
    border: '#2a2f3d',
    text: '#c4c9d4',
    textDim: '#6b7280',
    textBright: '#f0f2f5',
    coral: '#e8846b',
    mint: '#6bcf9e',
    rose: '#cf6b8a'
};

// Auto-refresh
(function() {
    const path = window.location.pathname;
    const isDashboard = path === '/' || path.endsWith('/');
    
    if (isDashboard) {
        let countdown = 30;
        const el = document.querySelector('.last-updated');
        
        if (el) {
            setInterval(() => {
                countdown--;
                el.textContent = countdown > 0 
                    ? `Refreshing in ${countdown}s` 
                    : 'Refreshing...';
            }, 1000);
        }
        
        setTimeout(() => location.reload(), 30000);
    }
})();

// Chart
function drawResponseChart() {
    const canvas = document.getElementById('responseChart');
    if (!canvas || typeof chartData === 'undefined') return;
    
    const ctx = canvas.getContext('2d');
    const { values, statuses } = chartData;
    
    if (values.length < 2) return;
    
    // Retina
    const dpr = window.devicePixelRatio || 1;
    const rect = canvas.getBoundingClientRect();
    canvas.width = rect.width * dpr;
    canvas.height = rect.height * dpr;
    ctx.scale(dpr, dpr);
    
    const width = rect.width;
    const height = rect.height;
    const pad = { top: 24, right: 16, bottom: 24, left: 48 };
    const chartW = width - pad.left - pad.right;
    const chartH = height - pad.top - pad.bottom;
    
    // Scale
    const validValues = values.filter(v => v > 0);
    let maxVal = validValues.length ? Math.max(...validValues) : 100;
    maxVal = Math.ceil(maxVal * 1.15 / 50) * 50;
    if (maxVal < 100) maxVal = 100;
    
    // Background
    ctx.fillStyle = theme.surface;
    ctx.fillRect(0, 0, width, height);
    
    // Grid
    ctx.strokeStyle = theme.border;
    ctx.lineWidth = 1;
    
    const gridLines = 4;
    ctx.font = '11px -apple-system, sans-serif';
    ctx.fillStyle = theme.textDim;
    ctx.textAlign = 'right';
    
    for (let i = 0; i <= gridLines; i++) {
        const y = pad.top + (chartH / gridLines) * i;
        const val = Math.round(maxVal - (maxVal / gridLines) * i);
        
        ctx.beginPath();
        ctx.moveTo(pad.left, y);
        ctx.lineTo(width - pad.right, y);
        ctx.stroke();
        
        ctx.fillText(val, pad.left - 8, y + 4);
    }
    
    const stepX = chartW / (values.length - 1);
    
    // Area gradient
    ctx.beginPath();
    ctx.moveTo(pad.left, pad.top + chartH);
    
    for (let i = 0; i < values.length; i++) {
        const x = pad.left + stepX * i;
        const y = pad.top + chartH - (values[i] / maxVal * chartH);
        ctx.lineTo(x, y);
    }
    
    ctx.lineTo(pad.left + chartW, pad.top + chartH);
    ctx.closePath();
    
    const grad = ctx.createLinearGradient(0, pad.top, 0, pad.top + chartH);
    grad.addColorStop(0, 'rgba(232, 132, 107, 0.25)');
    grad.addColorStop(1, 'rgba(232, 132, 107, 0)');
    ctx.fillStyle = grad;
    ctx.fill();
    
    // Line
    ctx.beginPath();
    ctx.strokeStyle = theme.coral;
    ctx.lineWidth = 2;
    ctx.lineJoin = 'round';
    ctx.lineCap = 'round';
    
    for (let i = 0; i < values.length; i++) {
        const x = pad.left + stepX * i;
        const y = pad.top + chartH - (values[i] / maxVal * chartH);
        i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y);
    }
    ctx.stroke();
    
    // Failure points
    for (let i = 0; i < values.length; i++) {
        if (statuses[i] === 'down') {
            const x = pad.left + stepX * i;
            const y = pad.top + chartH - (values[i] / maxVal * chartH);
            
            ctx.beginPath();
            ctx.fillStyle = theme.rose;
            ctx.arc(x, y, 5, 0, Math.PI * 2);
            ctx.fill();
        }
    }
}

document.addEventListener('DOMContentLoaded', () => {
    drawResponseChart();
    window.addEventListener('resize', () => {
        clearTimeout(window._resizeTimer);
        window._resizeTimer = setTimeout(drawResponseChart, 100);
    });
});
