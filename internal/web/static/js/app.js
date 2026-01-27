// Sentinel Dashboard JavaScript

// Theme colors (One Dark)
const theme = {
    bg: '#1e2127',
    bgElevated: '#282c34',
    border: '#3e4451',
    text: '#abb2bf',
    textMuted: '#5c6370',
    accent: '#e5e5e5',
    success: '#98c379',
    warning: '#e5c07b',
    danger: '#e06c75',
    info: '#61afef'
};

// Auto-refresh dashboard every 30 seconds
(function() {
    var path = window.location.pathname;
    var isDashboard = path === '/' || path.match(/\/$/);
    
    if (isDashboard) {
        var refreshInterval = 30000;
        var countdown = refreshInterval / 1000;
        
        var lastUpdated = document.querySelector('.last-updated');
        if (lastUpdated) {
            setInterval(function() {
                countdown--;
                if (countdown <= 0) {
                    lastUpdated.textContent = 'Refreshing...';
                } else {
                    lastUpdated.textContent = 'Auto-refresh in ' + countdown + 's';
                }
            }, 1000);
        }
        
        setTimeout(function() {
            window.location.reload();
        }, refreshInterval);
    }
})();

// Response time chart
function drawResponseChart() {
    var canvas = document.getElementById('responseChart');
    if (!canvas || typeof chartData === 'undefined') return;
    
    var ctx = canvas.getContext('2d');
    var data = chartData.values;
    var labels = chartData.labels;
    var statuses = chartData.statuses;
    
    if (data.length === 0) return;
    
    // Set canvas size for retina
    var dpr = window.devicePixelRatio || 1;
    var rect = canvas.getBoundingClientRect();
    canvas.width = rect.width * dpr;
    canvas.height = rect.height * dpr;
    ctx.scale(dpr, dpr);
    
    var width = rect.width;
    var height = rect.height;
    var padding = { top: 20, right: 20, bottom: 30, left: 50 };
    var chartWidth = width - padding.left - padding.right;
    var chartHeight = height - padding.top - padding.bottom;
    
    // Find max value
    var maxVal = Math.max.apply(null, data.filter(function(v) { return v > 0; }));
    if (maxVal === 0) maxVal = 100;
    maxVal = Math.ceil(maxVal * 1.1 / 100) * 100; // Round up to nearest 100
    
    // Clear canvas
    ctx.fillStyle = theme.bgElevated;
    ctx.fillRect(0, 0, width, height);
    
    // Draw grid lines
    ctx.strokeStyle = theme.border;
    ctx.lineWidth = 1;
    
    var gridLines = 4;
    for (var i = 0; i <= gridLines; i++) {
        var y = padding.top + (chartHeight / gridLines) * i;
        
        ctx.beginPath();
        ctx.moveTo(padding.left, y);
        ctx.lineTo(width - padding.right, y);
        ctx.stroke();
        
        // Y-axis labels
        var val = Math.round(maxVal - (maxVal / gridLines) * i);
        ctx.fillStyle = theme.textMuted;
        ctx.font = '11px SF Mono, Monaco, monospace';
        ctx.textAlign = 'right';
        ctx.fillText(val + 'ms', padding.left - 8, y + 4);
    }
    
    if (data.length < 2) return;
    
    var stepX = chartWidth / (data.length - 1);
    
    // Draw area fill
    ctx.beginPath();
    ctx.moveTo(padding.left, padding.top + chartHeight);
    
    for (var i = 0; i < data.length; i++) {
        var x = padding.left + stepX * i;
        var y = padding.top + chartHeight - (data[i] / maxVal * chartHeight);
        ctx.lineTo(x, y);
    }
    
    ctx.lineTo(padding.left + chartWidth, padding.top + chartHeight);
    ctx.closePath();
    
    var gradient = ctx.createLinearGradient(0, padding.top, 0, padding.top + chartHeight);
    gradient.addColorStop(0, 'rgba(97, 175, 239, 0.3)');
    gradient.addColorStop(1, 'rgba(97, 175, 239, 0)');
    ctx.fillStyle = gradient;
    ctx.fill();
    
    // Draw line
    ctx.beginPath();
    ctx.strokeStyle = theme.info;
    ctx.lineWidth = 2;
    ctx.lineJoin = 'round';
    ctx.lineCap = 'round';
    
    for (var i = 0; i < data.length; i++) {
        var x = padding.left + stepX * i;
        var y = padding.top + chartHeight - (data[i] / maxVal * chartHeight);
        
        if (i === 0) {
            ctx.moveTo(x, y);
        } else {
            ctx.lineTo(x, y);
        }
    }
    ctx.stroke();
    
    // Draw points for failures
    for (var i = 0; i < data.length; i++) {
        if (statuses[i] === 'down') {
            var x = padding.left + stepX * i;
            var y = padding.top + chartHeight - (data[i] / maxVal * chartHeight);
            
            ctx.beginPath();
            ctx.fillStyle = theme.danger;
            ctx.arc(x, y, 4, 0, Math.PI * 2);
            ctx.fill();
        }
    }
}

// Initialize
document.addEventListener('DOMContentLoaded', function() {
    drawResponseChart();
    
    // Redraw on resize
    var resizeTimer;
    window.addEventListener('resize', function() {
        clearTimeout(resizeTimer);
        resizeTimer = setTimeout(drawResponseChart, 100);
    });
});
