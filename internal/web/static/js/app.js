// Sentinel - Tactical Theme

const theme = {
    bg: '#0d0d0d',
    surface: '#1a1a1a',
    border: '#333333',
    text: '#e0e0e0',
    textDim: '#888888',
    textBright: '#ffffff',
    orange: '#ff6b35',
    statusUp: "#ffffff",
    statusDown: "#ff4444"
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
                    ? `NEXT SYNC IN ${countdown}S` 
                    : 'SYNCHRONIZING...';
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
    const pad = { top: 32, right: 20, bottom: 32, left: 56 };
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
    ctx.font = '600 10px Inter, sans-serif';
    ctx.fillStyle = theme.textDim;
    ctx.textAlign = 'right';
    
    for (let i = 0; i <= gridLines; i++) {
        const y = pad.top + (chartH / gridLines) * i;
        const val = Math.round(maxVal - (maxVal / gridLines) * i);
        
        ctx.beginPath();
        ctx.setLineDash([4, 4]);
        ctx.moveTo(pad.left, y);
        ctx.lineTo(width - pad.right, y);
        ctx.stroke();
        ctx.setLineDash([]);
        
        ctx.fillText(val + 'MS', pad.left - 10, y + 4);
    }
    
    const stepX = chartW / (values.length - 1);
    
    // Draw bars instead of area
    const barWidth = Math.max(2, Math.min(8, stepX - 2));
    
    for (let i = 0; i < values.length; i++) {
        const x = pad.left + stepX * i;
        const barHeight = values[i] / maxVal * chartH;
        const y = pad.top + chartH - barHeight;
        
        // Bar color based on status
        if (statuses[i] === 'up') {
            ctx.fillStyle = theme.statusUp;
        } else if (statuses[i] === 'down') {
            ctx.fillStyle = theme.statusDown;
        } else {
            ctx.fillStyle = theme.textDim;
        }
        
        ctx.fillRect(x - barWidth/2, y, barWidth, barHeight);
    }
    
    // Border
    ctx.strokeStyle = theme.border;
    ctx.lineWidth = 1;
    ctx.strokeRect(0.5, 0.5, width - 1, height - 1);
}

// Initialize chart when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', drawResponseChart);
} else {
    drawResponseChart();
}

// Redraw on resize
let resizeTimeout;
window.addEventListener('resize', () => {
    clearTimeout(resizeTimeout);
    resizeTimeout = setTimeout(drawResponseChart, 100);
});
