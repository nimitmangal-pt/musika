<library>
    <div class="row">
        <div class="col-xs-6 col-md-3" data-toggle="modal" data-target="#uploadModal">
            <a href="#" class="thumbnail">
            <img src="/coverArt?file={ file }" alt="{ Title }">
            Add files
            </a>
        </div>
        <div each={ items } class="col-xs-6 col-md-3" onclick={ add }>
            <a href="#" class="thumbnail">
            <img src="/coverArt?file={ file }" alt="{ Title }">
            </a>
        </div>
    </div>

    <script>
        add(e) {
            socket.emit("play media", e.item.file)
        }
    </script>
</library>